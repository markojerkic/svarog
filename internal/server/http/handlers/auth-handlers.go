package handlers

import (
	"fmt"
	"net/http"

	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/types"
	usercomponents "github.com/markojerkic/svarog/internal/server/ui/components/users"
	"github.com/markojerkic/svarog/internal/server/ui/pages/admin"
	authpages "github.com/markojerkic/svarog/internal/server/ui/pages/auth"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type AuthRouter struct {
	authService auth.AuthService
}

func (a *AuthRouter) getCurrentUser(c echo.Context) error {
	user, err := a.authService.GetCurrentUser(c)
	if err != nil {
		return c.JSON(401, types.ApiError{Message: "Not logged in"})
	}

	return c.JSON(200, user)
}

func (a *AuthRouter) login(c echo.Context) error {
	var loginForm types.LoginForm
	if err := c.Bind(&loginForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.LoginPage(authpages.LoginPageProps{
			Error:    "Invalid form data",
			Username: loginForm.Username,
			Password: loginForm.Password,
		}))
	}
	if err := c.Validate(&loginForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    "Username and password are required",
				Username: loginForm.Username,
				Password: loginForm.Password,
			},
		))
	}

	err := a.authService.Login(c, loginForm.Username, loginForm.Password)
	if err != nil {
		return utils.Render(c, http.StatusUnauthorized, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    "Invalid credentials",
				Username: loginForm.Username,
				Password: loginForm.Password,
			},
		))
	}

	if c.QueryParams().Has("redirect") {
		return utils.HxRedirect(c, c.QueryParams().Get("redirect"))
	}

	return utils.HxRedirect(c, "/")
}

func (a *AuthRouter) loginWithToken(c echo.Context) error {
	var loginForm types.LoginFormWithToken
	if err := c.Bind(&loginForm); err != nil {
		return c.JSON(400, err)
	}
	if err := c.Validate(&loginForm); err != nil {
		return err
	}

	err := a.authService.LoginWithToken(c, loginForm.Token)
	if err != nil {
		return c.JSON(401, types.ApiError{Message: "Invalid credentials"})
	}

	return c.JSON(200, "Logged in")
}

func (a *AuthRouter) loginPage(c echo.Context) error {
	return utils.Render(c, http.StatusOK, authpages.LoginPage())
}

func (a *AuthRouter) resetPasswordPage(c echo.Context) error {
	redirect := c.QueryParam("redirect")
	return utils.Render(c, http.StatusOK, authpages.ResetPasswordPage(authpages.ResetPasswordPageProps{
		Redirect: redirect,
	}))
}

func (a *AuthRouter) resetPassword(c echo.Context) error {
	var resetPasswordForm types.ResetPasswordForm
	if err := c.Bind(&resetPasswordForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError("Invalid form data"))
	}
	if err := c.Validate(&resetPasswordForm); err != nil {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError("Password must be at least 8 characters"))
	}

	if resetPasswordForm.Password != resetPasswordForm.RepeatedPassword {
		return utils.Render(c, http.StatusBadRequest, authpages.ResetPasswordError("Passwords do not match"))
	}

	user, err := a.authService.GetCurrentUser(c)
	if err != nil {
		return utils.HxRedirect(c, "/login")
	}

	err = a.authService.ResetPassword(c.Request().Context(), user.ID, resetPasswordForm)
	if err != nil {
		slog.Error("Error resetting password", "error", err)
		return utils.Render(c, http.StatusInternalServerError, authpages.ResetPasswordError("Error resetting password, try again"))
	}

	redirect := c.FormValue("redirect")
	if redirect != "" {
		return utils.HxRedirect(c, redirect)
	}
	return utils.HxRedirect(c, "/")
}

func (a *AuthRouter) logout(c echo.Context) error {
	err := a.authService.Logout(c)
	if err != nil {
		return c.JSON(500, types.ApiError{Message: "Error logging out"})
	}
	return utils.HxRedirect(c, "/")
}

func (a *AuthRouter) register(c echo.Context) error {
	var registerForm types.RegisterForm
	if err := c.Bind(&registerForm); err != nil {
		return c.JSON(400, err)
	}

	if err := c.Validate(&registerForm); err != nil {
		return err
	}

	loginToken, err := a.authService.Register(c, registerForm)
	if err != nil {
		if err.Error() == auth.UserAlreadyExists {
			return c.JSON(400, types.ApiError{Message: "User already exists", Fields: map[string]string{"username": "Username already exists"}})
		}
		return c.JSON(500, types.ApiError{Message: "Error registering user"})
	}

	return c.JSON(200, struct {
		LoginToken string `json:"loginToken"`
	}{
		LoginToken: loginToken,
	})
}

func (a *AuthRouter) getUsersPage(c echo.Context) error {
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
		TotalCount: totalCount,
	}))
}

func (a *AuthRouter) getEditUserForm(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "User ID is required"})
	}

	user, err := a.authService.GetUserByID(c.Request().Context(), id)
	if err != nil {
		slog.Error("Error fetching user", "error", err)
		if err.Error() == auth.ErrUserNotFound {
			return c.JSON(404, types.ApiError{Message: "User not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error getting user"})
	}

	return utils.Render(c, http.StatusOK, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
		FormID: "edit-user-form",
		Value: types.CreateUserForm{
			ID:        user.ID.Hex(),
			Username:  user.Username,
			FirstName: user.FirstName,
			LastName:  user.LastName,
			Role:      string(user.Role),
		},
	}))
}

func (a *AuthRouter) createOrUpdateUser(c echo.Context) error {
	var createUserForm types.CreateUserForm
	if err := c.Bind(&createUserForm); err != nil {
		return c.JSON(400, err)
	}
	if err := c.Validate(&createUserForm); err != nil {
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
					Message: "User already exists",
					Fields:  map[string]string{"username": "User with this username already exists"}},
				Value: createUserForm,
			}))
		}

		slog.Error("Error creating/updating user", "error", err)
		return utils.Render(c, http.StatusInternalServerError, usercomponents.NewUserForm(usercomponents.NewUserFormProps{
			ApiError: types.ApiError{
				Message: "Error creating/updating user",
			},
			Value: createUserForm,
		}))
	}

	htmx.CloseDialog(c)
	if createUserForm.ID != "" {
		htmx.AddSuccessToast(c, "User updated")
		htmx.Reswap(c, htmx.ReswapProps{
			Swap:   "outerHTML",
			Target: fmt.Sprintf("[data-user-id='%s']", createUserForm.ID),
			Select: "tr",
		})
	} else {
		htmx.AddSuccessToast(c, "User created")
	}

	return utils.Render(c, http.StatusOK, admin.UsersTableBody(admin.UsersListPageProps{
		Users: []auth.User{user},
	}))
}

func (a *AuthRouter) deleteUser(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(400, types.ApiError{Message: "User ID is required", Fields: map[string]string{"id": "User ID is required"}})
	}
	err := a.authService.DeleteUser(c, id)
	if err != nil {
		slog.Error("Error deleting user", "error", err)
		if err.Error() == auth.ErrUserNotFound {
			return c.JSON(404, types.ApiError{Message: "User not found"})
		}
		return c.JSON(500, types.ApiError{Message: "Error deleting user"})
	}

	htmx.AddSuccessToast(c, "User deleted")
	return c.HTML(200, "")
}

func NewAuthRouter(authService auth.AuthService,
	adminGroup *echo.Group,
	privateGroup *echo.Group,
	publicGroup *echo.Group) *AuthRouter {
	router := &AuthRouter{authService}

	if router.authService == nil {
		panic("No authService")
	}

	adminGroup.GET("/users", router.getUsersPage, middleware.RequiresRoleMiddleware(auth.ADMIN))
	adminGroup.GET("/users/:id/edit", router.getEditUserForm, middleware.RequiresRoleMiddleware(auth.ADMIN))
	adminGroup.POST("/users", router.createOrUpdateUser, middleware.RequiresRoleMiddleware(auth.ADMIN))
	adminGroup.DELETE("/users/:id", router.deleteUser, middleware.RequiresRoleMiddleware(auth.ADMIN))
	adminGroup.POST("/register", router.register, middleware.RequiresRoleMiddleware(auth.ADMIN))

	privateGroup.GET("/logout", router.logout)
	privateGroup.GET("/reset-password", router.resetPasswordPage)
	privateGroup.POST("/reset-password", router.resetPassword)

	publicGroup.GET("/login", router.loginPage)
	publicGroup.POST("/login", router.login)
	publicGroup.POST("/auth/login/token", router.loginWithToken)

	return router
}
