package handlers

import (
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	"github.com/markojerkic/svarog/internal/server/types"
)

type CertificateRouter struct {
	certificateService serverauth.CertificateService
	filesService       files.FileService
}

func (cr *CertificateRouter) generateCaCertificate(c echo.Context) error {
	err := cr.certificateService.GenerateCaCertificate(c.Request().Context())
	if err != nil {
		return c.JSON(500, types.ApiError{Message: err.Error()})
	}

	return c.JSON(200, "CA certificate generated")
}

func (cr *CertificateRouter) downloadCaCert(c echo.Context) error {
	caCrt, err := cr.filesService.GetFile(c.Request().Context(), "ca.crt")
	if err != nil {
		return c.JSON(500, types.ApiError{Message: err.Error()})
	}

	// Name attacment so it can download
	c.Response().Header().Add("Content-Disposition", "attachment")
	c.Response().Header().Add("filename", "ca.crt")

	return c.Blob(200, "application/octet-stream", caCrt)
}

func NewCertificateRouter(certificateService serverauth.CertificateService, filesService files.FileService, e *echo.Group) *CertificateRouter {
	router := &CertificateRouter{certificateService: certificateService, filesService: filesService}

	group := e.Group("/certificate")
	group.POST("/generate-ca", router.generateCaCertificate)
	group.GET("/ca", router.downloadCaCert)

	return router
}
