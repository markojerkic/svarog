package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/markojerkic/svarog/internal/server/http/middleware"
)

func TestAssetCacheControl(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "css"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "js"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "css", "output.css"), []byte("css"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "js", "log-menu.js"), []byte("app"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "js", "shadcn-templ-abc.js"), []byte("bundle"), 0o644))

	tests := []struct {
		name  string
		env   map[string]string
		path  string
		cache string
	}{
		{
			name:  "dev css",
			env:   map[string]string{"SHADCN_TEMPL_DEV": "true"},
			path:  "/assets/css/output.css",
			cache: "no-store",
		},
		{
			name:  "templ dev mode",
			env:   map[string]string{"TEMPL_DEV_MODE": "true"},
			path:  "/assets/js/log-menu.js",
			cache: "no-store",
		},
		{
			name:  "deprecated go env",
			env:   map[string]string{"GO_ENV": "development"},
			path:  "/assets/js/shadcn-templ-abc.js",
			cache: "no-store",
		},
		{
			name:  "production css revalidates",
			path:  "/assets/css/output.css",
			cache: "no-cache",
		},
		{
			name:  "production app js revalidates",
			path:  "/assets/js/log-menu.js",
			cache: "no-cache",
		},
		{
			name:  "production bundle is immutable",
			path:  "/assets/js/shadcn-templ-abc.js",
			cache: "public, max-age=31536000, immutable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SHADCN_TEMPL_DEV", "")
			t.Setenv("TEMPL_DEV_MODE", "")
			t.Setenv("GO_ENV", "")
			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			e := echo.New()
			e.Static("/assets", dir, middleware.AssetsCacheMiddleware)
			response := httptest.NewRecorder()
			e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))

			require.Equal(t, http.StatusOK, response.Code, "The asset file must be served from disk")
			require.Equal(t, tc.cache, response.Header().Get("Cache-Control"))
		})
	}
}
