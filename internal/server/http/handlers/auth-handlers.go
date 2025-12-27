package handlers

import (
	"net/http"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/types"
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
		return utils.Render(c, http.StatusOK, authpages.LoginPage(
			authpages.LoginPageProps{
				Error:    "Invalid credentials",
				Username: loginForm.Username,
				Password: loginForm.Password,
			},
		))
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
		log.Error("Error resetting password", "error", err)
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
	users, err := a.authService.GetUserPage(c.Request().Context(), query)
	if err != nil {
		return c.JSON(500, err)
	}
	return c.JSON(200, users)
}

func NewAuthRouter(authService auth.AuthService, privateGroup *echo.Group, publicGroup *echo.Group) *AuthRouter {
	router := &AuthRouter{authService}

	if router.authService == nil {
		panic("No authService")
	}

	privateGroup.GET("/current-user", router.getCurrentUser)
	privateGroup.GET("/users", router.getUsersPage, middleware.RequiresRoleMiddleware(auth.ADMIN))
	privateGroup.GET("/logout", router.logout)
	privateGroup.POST("/register", router.register, middleware.RequiresRoleMiddleware(auth.ADMIN))

	privateGroup.GET("/reset-password", router.resetPasswordPage)
	privateGroup.POST("/reset-password", router.resetPassword)

	publicGroup.GET("/login", router.loginPage)
	publicGroup.POST("/login", router.login)
	publicGroup.POST("/auth/login/token", router.loginWithToken)

	return router
}
