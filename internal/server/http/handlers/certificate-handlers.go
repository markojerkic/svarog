package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/types"
)

type CertificateRouter struct {
	certificateService serverauth.CertificateService
}

func (cr *CertificateRouter) GenerateCaCertificate(c echo.Context) error {
	err := cr.certificateService.GenerateCaCertificate(c.Request().Context())
	if err != nil {
		return c.JSON(500, types.ApiError{Message: err.Error()})
	}

	return c.JSON(200, "CA certificate generated")
}

func NewCertificateRouter(certificateService serverauth.CertificateService, e *echo.Group) *CertificateRouter {
	router := &CertificateRouter{certificateService: certificateService}

	group := e.Group("/certificate")
	group.POST("/generate-ca", router.GenerateCaCertificate)

	return router
}
