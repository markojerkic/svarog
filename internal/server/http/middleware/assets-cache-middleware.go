package middleware

import (
	"os"
	"strings"

	"github.com/labstack/echo/v5"
)

// AssetsCacheMiddleware sets Cache-Control for /assets.
// Development serves the files written by the Tailwind and script watchers and
// must not keep a copy from the previous process. Production keeps the hashed
// component bundle immutable and revalidates every other file.
func AssetsCacheMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", assetCacheControl(c.Request().URL.Path))
		return next(c)
	}
}

func assetCacheControl(path string) string {
	if assetDevMode() {
		return "no-store"
	}
	if strings.Contains(path, "/js/shadcn-templ-") {
		return "public, max-age=31536000, immutable"
	}
	return "no-cache"
}

func assetDevMode() bool {
	// GO_ENV=development is the deprecated alias shadcn-templ still accepts.
	return os.Getenv("SHADCN_TEMPL_DEV") == "true" ||
		os.Getenv("TEMPL_DEV_MODE") == "true" ||
		os.Getenv("GO_ENV") == "development"
}
