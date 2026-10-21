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
		return utils.HxRedirect(c, "/login")
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
	return utils.HxRedirect(c, "/")
}

func (a *AuthRouter) getUsersPage(c *echo.Context) error {
	var query types.GetUserPageInput
	if err := c.Bind(&query); err != nil {
		return c.JSON(400, err)
	}

	if query.Size == 0 {
		query.Size = 10
	}

	users, totalCount, err := a.authService.GetUserPage(c.Request().Context(), query)
	if err != nil {
		slog.Error("Error fetching users", "error", err)
		return err
	}
	return utils.Render(c, http.StatusOK, admin.UsersListPage(admin.UsersListPageProps{
		Users:      users,
		Page:       query.Page,
		Size:       query.Size,
		Search:     query.Search,
		TotalCount: totalCount,
	}))
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
	var createUserForm types.CreateUserForm
	if err := c.Bind(&createUserForm); err != nil {
		return c.JSON(400, err)
	}
	// Keep submitted assignments selected when the form is rendered with an error.
	for _, id := range createUserForm.ProjectIDs {
		createUserForm.Projects = append(createUserForm.Projects, combobox.Option{Value: id, Name: id})
	}
	if err := utils.Validate(c, &createUserForm); err != nil {
		if apiErr, ok := err.(types.ApiError); ok {
			htmx.Reswap(c, htmx.ReswapProps{
				Swap:   "outerHTML",
				Target: "this",
				Select: "form",
			})
			return utils.Render(c, http.StatusBadRequest, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
				ApiError: apiErr,
				Value:    createUserForm,
			}))
		}

		return err
	}

	slog.Debug("Creating user", "user", createUserForm)
	user, err := a.authService.CreateOrUpdateUser(c.Request().Context(), createUserForm)
	if err != nil {
		htmx.Reswap(c, htmx.ReswapProps{
			Swap:   "outerHTML",
			Target: "this",
			Select: "form",
		})
		if err.Error() == auth.UserAlreadyExists {
			return utils.Render(c, http.StatusConflict, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
				ApiError: types.ApiError{
					Message: i18n.T(c.Request().Context(), "users.already_exists"),
					Fields:  map[string]string{"username": i18n.T(c.Request().Context(), "users.username_exists")}},
				Value: createUserForm,
			}))
		}
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return utils.Render(c, http.StatusBadRequest, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
				ApiError: apiErr,
				Value:    createUserForm,
			}))
		}

		slog.Error("Error creating/updating user", "error", err)
		return utils.Render(c, http.StatusInternalServerError, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
			ApiError: types.ApiError{
				Message: i18n.T(c.Request().Context(), "users.save_failed"),
			},
			Value: createUserForm,
		}))
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

	return utils.Render(c, http.StatusOK, admin.UsersTableBody(admin.UsersListPageProps{
		Users: []auth.User{user},
	}))
}

func (a *AuthRouter) deleteUser(c *echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: i18n.T(c.Request().Context(), "users.id_required"), Fields: map[string]string{"id": i18n.T(c.Request().Context(), "users.id_required")}})
	}
	err := a.authService.DeleteUser(c, id)
	if err != nil {
		slog.Error("Error deleting user", "error", err)
		if err.Error() == auth.ErrUserNotFound {
			return c.JSON(404, types.ApiError{Message: i18n.T(c.Request().Context(), "users.not_found")})
		}
		return c.JSON(500, types.ApiError{Message: i18n.T(c.Request().Context(), "users.delete_failed")})
	}

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.deleted"))
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

	htmx.AddSuccessToast(c, i18n.T(c.Request().Context(), "users.token_copied"))
	htmx.AddTrigger(c, "copyLoginToken", loginToken)
	return c.String(200, loginToken)
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
