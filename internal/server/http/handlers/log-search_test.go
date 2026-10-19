package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

type logSearchStub struct {
	db.LogService
	request db.LogPageRequest
}

func (s *logSearchStub) GetLogs(_ context.Context, req db.LogPageRequest) (db.LogPage, error) {
	s.request = req
	return db.LogPage{
		Logs:       []types.StoredLog{{LogLine: "matching connection refused"}},
		IsLastPage: true,
	}, nil
}

func TestLogsPageSearch(t *testing.T) {
	for _, hx := range []bool{false, true} {
		for _, search := range []string{"++connection+refused++", "+++"} {
			t.Run(fmt.Sprintf("search=%s/htmx=%t", search, hx), func(t *testing.T) {
				e := echo.New()
				logs := &logSearchStub{}
				handlers.NewLogsRouter(logs,
					&projectAccessAuthStub{allowed: true},
					&projectAccessProjectsStub{belongs: true},
					loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}),
				)
				path := "/logs/" + projectID + "/" + clientID
				query := "?search=" + search + "&instance=api-1&instance=api-2&logLine=123"
				if strings.Contains(search, "connection") {
					// A new search must start at its first page, even with an old cursor in the URL.
					query += "&cursorTime=123&cursorSequenceNumber=2&direction=backward"
				}
				req := httptest.NewRequest(http.MethodGet, path+query, nil)
				if hx {
					req.Header.Set("HX-Request", "true")
				}
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)

				require.Equal(t, http.StatusOK, rec.Code, "search must render an HTML page")
				require.Contains(t, rec.Header().Get("Content-Type"), "text/html", "the UI must receive HTML rather than the JSON search response")
				require.Equal(t, projectID, logs.request.ProjectId, "search must use the authorized project")
				require.Equal(t, clientID, logs.request.ClientId, "search must use the authorized client")
				require.Equal(t, handlers.DEFAULT_PAGE_SIZE, logs.request.PageSize, "search must use the existing page size")
				require.Equal(t, &[]string{"api-1", "api-2"}, logs.request.Instances, "all active instance parameters must be retained")
				require.Nil(t, logs.request.Cursor, "this step must only fetch the first search page")
				require.Contains(t, rec.Body.String(), "matching connection refused", "the fetched log must be rendered")
				require.Contains(t, rec.Body.String(), `id="log-search-form"`, "HTMX and direct navigation must both return the complete view")
				if strings.Contains(search, "connection") {
					require.Equal(t, "connection refused", logs.request.Search, "the query must be trimmed before calling the service")
					require.Nil(t, logs.request.LogLineId, "a search must not inherit a log anchor")
					require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search results must not mount a live connection")
				} else {
					require.Empty(t, logs.request.Search, "whitespace-only queries must use the ordinary log view")
					require.Contains(t, rec.Body.String(), "hx-ws:connect=", "ordinary logs must retain their live connection")
				}
			})
		}
	}
}
