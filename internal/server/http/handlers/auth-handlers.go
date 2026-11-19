package handlers

import (
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
	authpages "github.com/markojerkic/svarog/internal/server/ui/pages/auth"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type AuthRouter struct {
	authService     auth.AuthService
	projectsService projects.ProjectsService
}

func loginRedirect(target string) string {
	u, err := url.Parse(target)
	if err != nil || u.IsAbs() || u.Host != "" || u.User != nil {
		return "/"
	}
	if !strings.HasPrefix(target, "/") || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") {
		return "/"
	}
	return target
}

func (a *AuthRouter) login(c *echo.Context) error {
	var loginForm forms.LoginForm
	bindErr := c.Bind(&loginForm)
	redirect := loginForm.Redirect
	if redirect == "" {
		redirect = c.QueryParam("redirect")
	}
	redirect = loginRedirect(redirect)
	if bindErr != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.LoginPage(authpages.LoginPageProps{
			Error:    i18n.T(c.Request().Context(), "auth.invalid_form"),
			Username: loginForm.Username,
			Password: loginForm.Password,
			Redirect: redirect,
		}))
	}
	if err := utils.Validate(c, &loginForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    i18n.T(c.Request().Context(), "auth.credentials_required"),
				Username: loginForm.Username,
				Password: loginForm.Password,
				Redirect: redirect,
			},
		))
	}

	err := a.authService.Login(c, loginForm.Username, loginForm.Password)
	if err != nil {
		return utils.Render(c, http.StatusUnauthorized, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    i18n.T(c.Request().Context(), "auth.invalid_credentials"),
				Username: loginForm.Username,
				Password: loginForm.Password,
				Redirect: redirect,
			},
		))
	}

	return utils.HxRedirect(c, redirect)
}

func (a *AuthRouter) loginWithToken(c *echo.Context) error {
	var loginForm forms.LoginFormWithToken
	if err := c.Bind(&loginForm); err != nil {
		return c.JSON(400, err)
	}
	if err := utils.Validate(c, &loginForm); err != nil {
		return err
	}

	err := a.authService.LoginWithToken(c, loginForm.Token)
	if err != nil {
		htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "auth.token_invalid"))
		return redirectAfterTokenLogin(c, "/login")
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "auth.logged_in"))
	return redirectAfterTokenLogin(c, "/reset-password")
}

// A 307 would replay POST /login/token onto the next page. HTMX keeps the
// existing toast plus HX-Redirect response.
func redirectAfterTokenLogin(c *echo.Context, target string) error {
	if htmx.IsHXRequest(c) {
		return htmx.Redirect(c, target)
	}
	return c.Redirect(http.StatusSeeOther, target)
}

// Preview clients fetch the login link before the user opens it. Rendering a
// confirmation form keeps that GET from spending the single-use token.
func (a *AuthRouter) loginTokenPage(c *echo.Context) error {
	header := c.Response().Header()
	header.Set("Cache-Control", "private, no-store")
	header.Set("Referrer-Policy", "no-referrer")
	return utils.Render(c, http.StatusOK, authpages.LoginTokenPage(authpages.LoginTokenPageProps{
		Token: c.QueryParam("token"),
	}))
}

func (a *AuthRouter) loginPage(c *echo.Context) error {
	return utils.Render(c, http.StatusOK, authpages.LoginPage(authpages.LoginPageProps{
		Redirect: loginRedirect(c.QueryParam("redirect")),
	}))
}

func (a *AuthRouter) resetPasswordPage(c *echo.Context) error {
	redirect := c.QueryParam("redirect")
	return utils.Render(c, http.StatusOK, authpages.ResetPasswordPage(authpages.ResetPasswordPageProps{
		Redirect: redirect,
	}))
}

func (a *AuthRouter) resetPassword(c *echo.Context) error {
	var resetPasswordForm forms.ResetPasswordForm
	if err := c.Bind(&resetPasswordForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.invalid_form")))
	}
	if err := utils.Validate(c, &resetPasswordForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.password_too_short")))
	}

	if resetPasswordForm.Password != resetPasswordForm.RepeatedPassword {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.passwords_mismatch")))
	}
	user, err := a.authService.GetCurrentUser(c)
	if err != nil {
		// Preserve the form error when a submitted password is invalid and the session lookup fails.
		if err := auth.ValidateNewPassword(resetPasswordForm.Password); err != nil {
			return resetPasswordError(c, err)
		}
		return htmx.Redirect(c, "/login")
	}

	err = a.authService.ResetPassword(c.Request().Context(), user.ID, resetPasswordForm.Password)
	if err != nil {
		return resetPasswordError(c, err)
	}

	redirect := c.FormValue("redirect")
	if redirect != "" {
		return utils.HxRedirect(c, redirect)
	}
	return utils.HxRedirect(c, "/")
}

func resetPasswordError(c *echo.Context, err error) error {
	status := http.StatusBadRequest
	var key string
	switch {
	case errors.Is(err, auth.ErrPasswordBlank):
		key = "auth.password_blank"
	case errors.Is(err, auth.ErrPasswordTooLong):
		key = "auth.password_too_short"
	default:
		slog.Error("Error resetting password", "error", err)
		status = http.StatusInternalServerError
		key = "auth.reset_failed"
	}
	return utils.Render(c, status, authpages.ResetPasswordError(i18n.T(c.Request().Context(), key)))
}

func (a *AuthRouter) logout(c *echo.Context) error {
	err := a.authService.Logout(c)
	if err != nil {
		return c.JSON(500, forms.ApiError{Message: i18n.T(c.Request().Context(), "auth.logout_failed")})
	}
	return htmx.Redirect(c, "/login")
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

func NewAuthRouter(authService auth.AuthService,
	projectsService projects.ProjectsService,
	adminGroup *echo.Group,
	privateGroup *echo.Group,
	publicGroup *echo.Group) *AuthRouter {
	router := &AuthRouter{authService: authService, projectsService: projectsService}

	if router.authService == nil {
		panic("No authService")
	}

	adminGroup.GET("/users", router.getUsersPage)
	adminGroup.GET("/users/:id/edit", router.getEditUserForm)
	adminGroup.POST("/users", router.createOrUpdateUser)
	adminGroup.DELETE("/users/:id", router.deleteUser)
	adminGroup.POST("/users/generate-login-token", router.generateLoginToken)

	privateGroup.GET("/logout", router.logout)
	privateGroup.GET("/reset-password", router.resetPasswordPage)
	privateGroup.POST("/reset-password", router.resetPassword)

	publicGroup.GET("/login", func(c *echo.Context) error {
		if c.QueryParams().Has("token") {
			return router.loginTokenPage(c)
		}
		return router.loginPage(c)
	})
	publicGroup.POST("/login", router.login)
	publicGroup.POST("/login/token", router.loginWithToken)

	return router
}
