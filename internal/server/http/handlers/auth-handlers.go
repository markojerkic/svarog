package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/forms"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/components/combobox"
	usercomponents "github.com/markojerkic/svarog/internal/server/ui/components/users"
	"github.com/markojerkic/svarog/internal/server/ui/pages/admin"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type userAdminService interface {
	GetUserByID(context.Context, string) (auth.User, error)
	GetUserPage(context.Context, auth.PageQuery) ([]auth.User, int64, error)
	CreateUser(context.Context, auth.UserInput) (auth.User, string, error)
	UpdateUser(context.Context, uuid.UUID, auth.UserInput) (auth.User, error)
	DeleteUser(context.Context, string) error
	GenerateLoginToken(context.Context, string) (string, error)
}

type assignedProjects interface {
	ByIDs(context.Context, []string) ([]projects.Project, error)
}

type AuthRouter struct {
	authService     userAdminService
	projectsService assignedProjects
}

func (a *AuthRouter) getUsersPage(c *echo.Context) error {
	var query forms.PageInput
	if err := c.Bind(&query); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}

	if query.Size == 0 {
		query.Size = 10
	}

	if err := utils.Validate(c, &query); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	pageQuery, err := auth.NewPageQuery(query.Search, query.Page, query.Size)
	if err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	return a.renderUsersPage(c, pageQuery, false, "")
}

func (a *AuthRouter) getEditUserForm(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, forms.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required")})
	}

	user, err := a.authService.GetUserByID(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching user", "error", err)
		return userOperationError(c, err, "users.load_failed")
	}
	var projectItems []combobox.Option
	if len(user.ProjectIDs) > 0 {
		assignedProjects, err := a.projectsService.ByIDs(c.Request().Context(), user.ProjectIDs)
		if err != nil {
			slog.Error("Error fetching assigned projects", "error", err)
			return c.JSON(http.StatusInternalServerError, forms.ApiError{Message: i18n.T(c.Request().Context(), "projects.selected_load_failed")})
		}
		for _, project := range assignedProjects {
			projectItems = append(projectItems, combobox.Option{Value: project.ID.String(), Name: project.Name})
		}
	}

	return utils.Render(c, http.StatusOK, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
		FormID:   "edit-user-form",
		Projects: projectItems,
		Value: forms.CreateUserForm{
			ID:         user.ID.String(),
			Username:   user.Username,
			FirstName:  user.FirstName,
			LastName:   user.LastName,
			Role:       string(user.Role),
			ProjectIDs: forms.CommaSeparatedStrings(user.ProjectIDs),
		},
	}))
}

func (a *AuthRouter) createOrUpdateUser(c *echo.Context) error {
	query, err := userMutationQuery(c)
	if err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	var createUserForm forms.CreateUserForm
	if err := c.Bind(&createUserForm); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	if err := utils.Validate(c, &createUserForm); err != nil {
		var apiErr forms.ApiError
		if !errors.As(err, &apiErr) {
			apiErr = forms.NewApiError(i18n.T(c.Request().Context(), "validation.failed"))
		}
		return a.renderUserFormError(c, http.StatusBadRequest, createUserForm, apiErr)
	}

	ctx := c.Request().Context()
	input := auth.UserInput{
		Username: createUserForm.Username, FirstName: createUserForm.FirstName, LastName: createUserForm.LastName,
		Role: auth.Role(createUserForm.Role), ProjectIDs: make([]uuid.UUID, 0, len(createUserForm.ProjectIDs)),
	}
	for _, value := range createUserForm.ProjectIDs {
		input.ProjectIDs = append(input.ProjectIDs, canonicalFormUUID(value))
	}
	var user auth.User
	var initialLoginToken string
	if createUserForm.ID == "" {
		user, initialLoginToken, err = a.authService.CreateUser(ctx, input)
	} else {
		user, err = a.authService.UpdateUser(ctx, canonicalFormUUID(createUserForm.ID), input)
	}
	if err != nil {
		if errors.Is(err, auth.ErrUserExists) {
			return a.renderUserFormError(c, http.StatusConflict, createUserForm, forms.NewApiError(
				i18n.T(c.Request().Context(), "users.already_exists"), map[string]string{"username": i18n.T(c.Request().Context(), "users.username_exists")},
			))
		}
		if errors.Is(err, auth.ErrUserNotFound) {
			return a.renderUserFormError(c, http.StatusNotFound, createUserForm, forms.NewApiError(i18n.T(c.Request().Context(), "users.not_found")))
		}
		if errors.Is(err, auth.ErrProjectSelectionMissing) {
			ctx := c.Request().Context()
			return a.renderUserFormError(c, http.StatusBadRequest, createUserForm, forms.NewApiError(
				i18n.T(ctx, "projects.not_found"), map[string]string{"projects": i18n.T(ctx, "projects.selection_missing")},
			))
		}
		if apiErr, ok := validationResponse(c.Request().Context(), err); ok {
			return a.renderUserFormError(c, http.StatusBadRequest, createUserForm, apiErr)
		}

		slog.Error("Error creating/updating user", "error", err)
		return userPageError(c, http.StatusInternalServerError, i18n.T(c.Request().Context(), "users.save_failed"))
	}

	htmx.CloseDialog(c)
	if createUserForm.ID != "" {
		htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.updated"))
		htmx.Reswap(c, htmx.ReswapProps{
			Swap:   "outerHTML",
			Target: fmt.Sprintf("[data-user-id='%s']", createUserForm.ID),
			Select: "tr",
		})
	} else {
		htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.created"))
	}
	if htmx.IsHXRequest(c) {
		loginURL := ""
		if createUserForm.ID == "" && initialLoginToken != "" {
			loginURL = loginLinkURL(c, initialLoginToken)
		}
		return a.renderUsersPage(c, query, true, loginURL)
	}

	return utils.Render(c, http.StatusOK, admin.UsersTableBody(admin.UsersListPageProps{
		Users: []auth.User{user},
	}))
}

func (a *AuthRouter) deleteUser(c *echo.Context) error {
	query, err := userMutationQuery(c)
	if err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, forms.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required"), Fields: map[string]string{"id": i18n.T(c.Request().Context(), "users.id_required")}})
	}
	err = a.authService.DeleteUser(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error deleting user", "error", err)
		return userOperationError(c, err, "users.delete_failed")
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.deleted"))
	if htmx.IsHXRequest(c) {
		return a.renderUsersPage(c, query, true, "")
	}
	return c.HTML(200, "")
}

func userOperationError(c *echo.Context, err error, failureKey string) error {
	ctx := c.Request().Context()
	if errors.Is(err, auth.ErrUserNotFound) {
		return c.JSON(http.StatusNotFound, forms.ApiError{Message: i18n.T(ctx, "users.not_found")})
	}
	return c.JSON(http.StatusInternalServerError, forms.ApiError{Message: i18n.T(ctx, failureKey)})
}

func (a *AuthRouter) generateLoginToken(c *echo.Context) error {
	userId := c.FormValue("userId")
	if userId == "" {
		return c.JSON(400, forms.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required")})
	}

	loginToken, err := a.authService.GenerateLoginToken(c.Request().Context(), userId)
	if err != nil {
		slog.Error("Error generating login token", "error", err)
		return err
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.login_link_created"))
	htmx.Reswap(c, htmx.ReswapProps{
		Swap:   "innerHTML",
		Target: "#login-link-result",
	})
	return utils.Render(c, http.StatusOK, usercomponents.LoginLinkResult(loginLinkURL(c, loginToken)))
}

func requestScheme(c *echo.Context) string {
	if c.Scheme() == "https" {
		return "https"
	}
	if strings.EqualFold(strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto")), "https") {
		return "https"
	}
	return "http"
}

func loginLinkURL(c *echo.Context, token string) string {
	return requestScheme(c) + "://" + c.Request().Host + "/login?token=" + url.QueryEscape(token)
}

func NewAuthRouter(authService userAdminService,
	projectsService assignedProjects,
	adminGroup *echo.Group) *AuthRouter {
	router := &AuthRouter{authService: authService, projectsService: projectsService}

	if router.authService == nil {
		panic("No authService")
	}

	adminGroup.GET("/users", router.getUsersPage)
	adminGroup.GET("/users/:id/edit", router.getEditUserForm)
	adminGroup.POST("/users", router.createOrUpdateUser)
	adminGroup.DELETE("/users/:id", router.deleteUser)
	adminGroup.POST("/users/generate-login-token", router.generateLoginToken)

	return router
}
