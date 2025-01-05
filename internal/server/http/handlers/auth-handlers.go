package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/types"
)

type AuthRouter struct {
	authService auth.AuthService
}

type LoginForm struct {
	Username string `json:"username" form:"username" validate:"required,gte=3"`
	Password string `json:"password" form:"password" validate:"required,gte=8"`
}

func (a *AuthRouter) getCurrentUser(c echo.Context) error {
	user, err := a.authService.GetCurrentUser(c)
	if err != nil {
		return c.JSON(401, types.ApiError{Message: "Not logged in"})
	}

	return c.JSON(200, user)
}

func (a *AuthRouter) login(c echo.Context) error {
	var loginForm LoginForm
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
	group.GET("/current-user", router.getCurrentUser)
	group.GET("/users", router.getUsersPage)

	return router
}
