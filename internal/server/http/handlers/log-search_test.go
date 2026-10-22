package handlers_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/require"
)

type logSearchStub struct {
	db.LogService
	request       db.LogPageRequest
	instanceCalls int
	logCalls      int
}

func (s *logSearchStub) GetInstances(_ context.Context, project, client string) ([]string, error) {
	s.instanceCalls++
	if project != s.request.ProjectId || client != s.request.ClientId {
		return nil, fmt.Errorf("instances must use the same scope as the logs")
	}
	return []string{"api-1", "api-2", "api-3"}, nil
}

func (s *logSearchStub) GetLogs(_ context.Context, req db.LogPageRequest) (db.LogPage, error) {
	s.logCalls++
	s.request = req
	return db.LogPage{
		Logs:       []types.StoredLog{{LogLine: "matching connection refused"}},
		IsLastPage: req.Cursor == nil,
	}, nil
}

func (s *logSearchStub) SearchLogs(ctx context.Context, search, project, client string, instances *[]string, pageSize int64, cursor *db.LastCursor) ([]types.StoredLog, error) {
	page, err := s.GetLogs(ctx, db.LogPageRequest{
		ProjectId: project, ClientId: client, Search: search, Instances: instances,
		PageSize: pageSize, Cursor: cursor,
	})
	return page.Logs, err
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
				require.Nil(t, logs.request.Cursor, "a query without cursor parameters must fetch the first page")
				require.Contains(t, rec.Body.String(), "matching connection refused", "the fetched log must be rendered")
				require.Contains(t, rec.Body.String(), `id="log-search-form"`, "HTMX and direct navigation must both return the complete view")
				if strings.Contains(search, "connection") {
					require.Equal(t, 1, logs.instanceCalls, "search must load the client's instance choices")
					require.Contains(t, rec.Body.String(), "api-3", "filter choices must include instances outside the current selection")
					require.NotContains(t, rec.Body.String(), "Filtered by instance:", "search must keep instance controls inside the popover, not in a permanent row")
					require.Equal(t, "connection refused", logs.request.Search, "the query must be trimmed before calling the service")
					require.Nil(t, logs.request.LogLineId, "a search must not inherit a log anchor")
					require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search results must not mount a live connection")
				} else {
					require.Zero(t, logs.instanceCalls, "ordinary logs must not load search-only filter choices")
					require.Empty(t, logs.request.Search, "whitespace-only queries must use the ordinary log view")
					require.NotNil(t, logs.request.LogLineId, "whitespace-only queries must retain the log anchor")
					require.Equal(t, "123", *logs.request.LogLineId, "whitespace-only queries must preserve the log anchor")
					require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "instance-filtered and anchored views must not mount a live connection")
				}
			})
		}
	}
}

func TestLogsPageLiveModes(t *testing.T) {
	for _, requestType := range []string{"", "full", "partial"} {
		for _, tc := range []struct {
			name, query string
			live        bool
		}{
			{"live", "", true},
			{"blank search", "?search=+++", true},
			{"search", "?search=error", false},
			{"instance", "?instance=api-1", false},
			{"anchor", "?logLine=42", false},
			{"cursor", "?cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7&direction=backward", false},
		} {
			t.Run(tc.name+"/request="+requestType, func(t *testing.T) {
				e := echo.New()
				logs := &logSearchStub{}
				handlers.NewLogsRouter(logs,
					&projectAccessAuthStub{allowed: true},
					&projectAccessProjectsStub{belongs: true},
					loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}),
				)
				path := "/logs/" + projectID + "/" + clientID
				req := httptest.NewRequest(http.MethodGet, path+tc.query, nil)
				if requestType != "" {
					req.Header.Set("HX-Request", "true")
					req.Header.Set("HX-Request-Type", requestType)
				}
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				require.Equal(t, http.StatusOK, rec.Code, "every log mode must render successfully")
				require.Equal(t, tc.live, strings.Contains(rec.Body.String(), "hx-ws:connect="), "only the latest unfiltered, unanchored view may mount a live connection")
				if requestType == "partial" && logs.request.Cursor != nil {
					require.NotContains(t, rec.Body.String(), "<header", "pagination fragments must not replace the page header")
				} else {
					require.Equal(t, !tc.live, strings.Contains(rec.Body.String(), `aria-label="Back to live"`), "every non-live page must offer a return to the live view")
					status := "Not live"
					if logs.request.LogLineId != nil {
						status = "Line context"
					}
					require.Equal(t, !tc.live, strings.Contains(rec.Body.String(), status), "the header must reflect the same live state as the connection")
				}
			})
		}
	}
}

func TestLogCursorValidation(t *testing.T) {
	for _, route := range []string{"", "/search"} {
		for _, tc := range []struct {
			name   string
			query  string
			status int
			micros int64
		}{
			{"microseconds", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7&direction=backward", 200, 1791979200123456},
			{"legacy milliseconds", "cursorTime=1791979200123&cursorSequenceNumber=2&direction=forward", 200, 1791979200123000},
			{"missing direction", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2", 400, 0},
			{"invalid direction", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&direction=sideways", 400, 0},
			{"missing sequence", "cursorTimeUs=1791979200123456&direction=backward", 400, 0},
			{"missing timestamp", "cursorSequenceNumber=2&direction=backward", 400, 0},
			{"ambiguous timestamp", "cursorTime=123&cursorTimeUs=123000&cursorSequenceNumber=2&direction=backward", 400, 0},
			{"nonpositive timestamp", "cursorTimeUs=0&cursorSequenceNumber=2&direction=backward", 400, 0},
			{"invalid timestamp", "cursorTimeUs=invalid&cursorSequenceNumber=2&direction=backward", 400, 0},
			{"timestamp overflow", "cursorTime=9223372036854775807&cursorSequenceNumber=2&direction=backward", 400, 0},
			{"invalid id", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=-1&direction=backward", 400, 0},
		} {
			t.Run(route+"/"+tc.name, func(t *testing.T) {
				e := echo.New()
				logs := &logSearchStub{}
				handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true}, loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
				req := httptest.NewRequest(http.MethodGet, "/logs/"+projectID+"/"+clientID+route+"?search=connection&"+tc.query, nil)
				req.Header.Set("HX-Request", "true")
				req.Header.Set("HX-Request-Type", "partial")
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				require.Equal(t, tc.status, rec.Code, "both log endpoints must validate cursor parameters")
				if tc.status != http.StatusOK {
					require.Zero(t, logs.logCalls, "invalid cursors must be rejected before querying logs")
					return
				}
				require.Equal(t, time.UnixMicro(tc.micros), logs.request.Cursor.Timestamp, "cursor parsing must preserve timestamp precision and accept legacy links")
				require.Equal(t, 2, logs.request.Cursor.SequenceNumber, "cursor parsing must preserve the sequence number")
				require.Equal(t, strings.Contains(tc.query, "direction=backward"), logs.request.Cursor.IsBackward, "cursor parsing must preserve the direction")
				if strings.Contains(tc.query, "cursorId=7") {
					require.Equal(t, int64(7), logs.request.Cursor.ID, "cursor parsing must preserve the row id tie breaker")
				}
				require.Zero(t, logs.instanceCalls, "pagination fragments must not reload filter choices")
				if route == "" {
					require.NotContains(t, rec.Body.String(), "<html", "pagination must return only log rows")
					require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search pagination must never connect a WebSocket")
				} else {
					require.Contains(t, rec.Header().Get("Content-Type"), "application/json", "the existing search API must keep its JSON response")
				}
			})
		}
	}
}

func TestSearchCursorFullPage(t *testing.T) {
	for _, requestType := range []string{"", "full"} {
		t.Run(requestType, func(t *testing.T) {
			e := echo.New()
			logs := &logSearchStub{}
			handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true}, loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
			req := httptest.NewRequest(http.MethodGet, "/logs/"+projectID+"/"+clientID+"?search=connection&instance=api-1&instance=api-2&cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7&direction=backward", nil)
			if requestType != "" {
				req.Header.Set("HX-Request", "true")
				req.Header.Set("HX-Request-Type", requestType)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code, "direct and full HTMX navigation must support cursor URLs")
			require.Contains(t, rec.Body.String(), `id="log-search-form"`, "full navigation must return the complete search view")
			require.Equal(t, &[]string{"api-1", "api-2"}, logs.request.Instances, "pagination must retain every selected instance")
			require.Equal(t, 1, logs.instanceCalls, "full search pages must load filter choices")
			require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search pages opened at a cursor must remain non-live")
		})
	}
}
