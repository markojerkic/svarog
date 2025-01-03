package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/auth"
)

type AuthRouter struct {
	authService auth.AuthService
}

type LoginForm struct {
	Username string `json:"username" form:"username"`
	Password string `json:"password" form:"password"`
}

func (a *AuthRouter) getCurrentUser(c echo.Context) error {
	user, err := a.authService.GetCurrentUser(c)
	if err != nil {
		return c.JSON(500, err)
	}

	return c.JSON(200, user)
}

func (a *AuthRouter) login(c echo.Context) error {
	var loginForm LoginForm
	if err := c.Bind(&loginForm); err != nil {
		return c.JSON(400, err)
	}

	err := a.authService.Login(c, loginForm.Username, loginForm.Password)
	if err != nil {
		return c.JSON(500, err)
	}

	return c.JSON(200, "Logged in")
}

func NewAuthRouter(authService auth.AuthService, e *echo.Group) *AuthRouter {
	router := &AuthRouter{authService}

	group := e.Group("/auth")
	group.POST("/login", router.login)
	group.GET("/current-user", router.getCurrentUser)

	return router
}
