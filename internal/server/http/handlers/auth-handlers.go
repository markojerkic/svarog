package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
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
	var loginForm types.LoginForm
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
	var loginForm types.LoginFormWithToken
	if err := c.Bind(&loginForm); err != nil {
		return c.JSON(400, err)
	}
	if err := utils.Validate(c, &loginForm); err != nil {
		return err
	}

	err := a.authService.LoginWithToken(c, loginForm.Token)
	if err != nil {
		htmx.AddErrorToast(c, i18n.T(c.Request().Context(), "auth.token_invalid"))
		return htmx.Redirect(c, "/login")
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "auth.logged_in"))
	return htmx.Redirect(c, "/reset-password")
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
	var resetPasswordForm types.ResetPasswordForm
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
		return htmx.Redirect(c, "/login")
	}

	err = a.authService.ResetPassword(c.Request().Context(), user.ID, resetPasswordForm)
	if err != nil {
		slog.Error("Error resetting password", "error", err)
		return utils.Render(c, http.StatusInternalServerError, authpages.ResetPasswordError(i18n.T(c.Request().Context(), "auth.reset_failed")))
	}

	redirect := c.FormValue("redirect")
	if redirect != "" {
		return utils.HxRedirect(c, redirect)
	}
	return utils.HxRedirect(c, "/")
}

func (a *AuthRouter) logout(c *echo.Context) error {
	err := a.authService.Logout(c)
	if err != nil {
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "auth.logout_failed")})
	}
	return htmx.Redirect(c, "/login")
}

func (a *AuthRouter) getUsersPage(c *echo.Context) error {
	var query types.GetUserPageInput
	if err := c.Bind(&query); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}

	if query.Size == 0 {
		query.Size = 10
	}

	if err := utils.Validate(c, &query); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	if err := auth.ValidateUserPage(c.Request().Context(), query); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	return a.renderUsersPage(c, query, false)
}

func (a *AuthRouter) getEditUserForm(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required")})
	}

	user, err := a.authService.GetUserByID(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching user", "error", err)
		if err.Error() == auth.ErrUserNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "users.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "users.load_failed")})
	}
	var projectItems []combobox.Option
	if len(user.ProjectIDs) > 0 {
		assignedProjects, err := a.projectsService.GetProjectsByIds(c.Request().Context(), user.ProjectIDs)
		if err != nil {
			slog.Error("Error fetching assigned projects", "error", err)
			return c.JSON(http.StatusInternalServerError, types.ApiError{Message: i18n.T(c.Request().Context(), "projects.selected_load_failed")})
		}
		for _, project := range assignedProjects {
			projectItems = append(projectItems, combobox.Option{Value: project.ID.String(), Name: project.Name})
		}
	}

	return utils.Render(c, http.StatusOK, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
		FormID: "edit-user-form",
		Value: types.CreateUserForm{
			ID:         user.ID.String(),
			Username:   user.Username,
			FirstName:  user.FirstName,
			LastName:   user.LastName,
			Role:       string(user.Role),
			ProjectIDs: types.CommaSeparatedStrings(user.ProjectIDs),
			Projects:   projectItems,
		},
	}))
}

func (a *AuthRouter) createOrUpdateUser(c *echo.Context) error {
	query, err := userMutationQuery(c)
	if err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	var createUserForm types.CreateUserForm
	if err := c.Bind(&createUserForm); err != nil {
		return userPageError(c, http.StatusBadRequest, i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	if err := utils.Validate(c, &createUserForm); err != nil {
		var apiErr types.ApiError
		if !errors.As(err, &apiErr) {
			apiErr = types.NewApiError(i18n.T(c.Request().Context(), "validation.failed"))
		}
		return a.renderUserFormError(c, http.StatusBadRequest, createUserForm, apiErr)
	}
	if err := auth.ValidateUserForm(c.Request().Context(), createUserForm); err != nil {
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return a.renderUserFormError(c, http.StatusBadRequest, createUserForm, apiErr)
		}
		return err
	}

	slog.Debug("Creating user", "user", createUserForm)
	user, err := a.authService.CreateOrUpdateUser(c.Request().Context(), createUserForm)
	if err != nil {
		if err.Error() == auth.UserAlreadyExists {
			return a.renderUserFormError(c, http.StatusConflict, createUserForm, types.NewApiError(
				i18n.T(c.Request().Context(), "users.already_exists"), map[string]string{"username": i18n.T(c.Request().Context(), "users.username_exists")},
			))
		}
		if err.Error() == auth.ErrUserNotFound {
			return a.renderUserFormError(c, http.StatusNotFound, createUserForm, types.NewApiError(i18n.T(c.Request().Context(), "users.not_found")))
		}
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
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
		return a.renderUsersPage(c, query, true)
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
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required"), Fields: map[string]string{"id": i18n.T(c.Request().Context(), "users.id_required")}})
	}
	err = a.authService.DeleteUser(c, id)
	if err != nil {
		slog.Error("Error deleting user", "error", err)
		if err.Error() == auth.ErrUserNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "users.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "users.delete_failed")})
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.deleted"))
	if htmx.IsHXRequest(c) {
		return a.renderUsersPage(c, query, true)
	}
	return c.HTML(200, "")
}

func (a *AuthRouter) generateLoginToken(c *echo.Context) error {
	userId := c.FormValue("userId")
	if userId == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required")})
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
			return router.loginWithToken(c)
		}
		return router.loginPage(c)
	})
	publicGroup.POST("/login", router.login)

	return router
}
