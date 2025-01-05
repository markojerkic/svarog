package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
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
		return c.JSON(400, err)
	}
	if err := c.Validate(&loginForm); err != nil {
		return err
	}

	err := a.authService.Login(c, loginForm.Username, loginForm.Password)
	if err != nil {
		return c.JSON(401, types.ApiError{Message: "Invalid credentials"})
	}

	return c.JSON(200, "Logged in")
}

func (a *AuthRouter) register(c echo.Context) error {
	var registerForm types.RegisterForm
	if err := c.Bind(&registerForm); err != nil {
		return c.JSON(400, err)
	}
	if err := c.Validate(&registerForm); err != nil {
		return err
	}

	err := a.authService.Register(c, registerForm)
	if err != nil {
		if err.Error() == auth.UserAlreadyExists {
			return c.JSON(400, types.ApiError{Message: "User already exists"})
		}
		return c.JSON(500, types.ApiError{Message: "Error registering user"})
	}

	return c.JSON(200, "Logged in")
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

func NewAuthRouter(authService auth.AuthService, e *echo.Group) *AuthRouter {
	router := &AuthRouter{authService}

	if router.authService == nil {
		panic("No authService")
	}

	group := e.Group("/auth")
	group.POST("/login", router.login)
	group.POST("/register", router.register)
	group.GET("/current-user", router.getCurrentUser)
	group.GET("/users", router.getUsersPage)

	return router
}
