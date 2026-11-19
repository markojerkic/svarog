package handlers_test

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/stretchr/testify/require"
)

type logSearchStub struct {
	request       logstore.PageQuery
	instanceCalls int
	logCalls      int
	anchorID      int64
	page          *logstore.Page
}

func (s *logSearchStub) Instances(_ context.Context, scope logstore.Scope) ([]string, error) {
	s.instanceCalls++
	if scope != s.request.Scope {
		return nil, fmt.Errorf("instances must use the same scope as the logs")
	}
	return []string{"api-1", "api-2", "api-3"}, nil
}

func (s *logSearchStub) Page(_ context.Context, request logstore.PageQuery) (logstore.Page, error) {
	s.logCalls++
	s.request = request
	return logstore.Page{Rows: []logstore.Row{{Message: "matching connection refused"}}}, nil
}

func (s *logSearchStub) Around(_ context.Context, scope logstore.Scope, id int64, limit int, bounds *logstore.TimeRange) (logstore.Page, bool, error) {
	s.logCalls++
	s.anchorID = id
	s.request = logstore.PageQuery{Scope: scope, Limit: limit, Range: bounds}
	if s.page != nil {
		return *s.page, true, nil
	}
	return logstore.Page{}, false, nil
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
				require.Contains(t, rec.Header().Get("Content-Type"), "text/html", "the log page must return HTML")
				require.Equal(t, uuid.MustParse(projectID), logs.request.Scope.ProjectID, "search must use the authorized project")
				require.Equal(t, uuid.MustParse(clientID), logs.request.Scope.ClientID, "search must use the authorized client")
				require.Equal(t, handlers.DEFAULT_PAGE_SIZE, logs.request.Limit, "search must use the existing page size")
				require.Equal(t, []string{"api-1", "api-2"}, logs.request.Instances, "all active instance parameters must be retained")
				require.Nil(t, logs.request.Older, "the first page has no older cursor")
				require.Nil(t, logs.request.Newer, "the first page has no newer cursor")
				require.Contains(t, rec.Body.String(), "matching connection refused", "the fetched log must be rendered")
				require.Contains(t, rec.Body.String(), `id="log-search-form"`, "HTMX and direct navigation must both return the complete view")
				if strings.Contains(search, "connection") {
					require.Equal(t, 1, logs.instanceCalls, "search must load the client's instance choices")
					require.Contains(t, rec.Body.String(), "api-3", "filter choices must include instances outside the current selection")
					require.Contains(t, rec.Body.String(), "Filtered by instance:", "search must show active instances without opening the popover")
					require.Equal(t, []string{"connection", "refused"}, logs.request.Search.Terms, "the handler must pass parsed search terms to the reader")
					require.Zero(t, logs.anchorID, "search must not load a log anchor")
					require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search results must not mount a live connection")
				} else {
					require.Equal(t, 1, logs.instanceCalls, "ordinary logs must load the complete client instance list for consistent colors")
					require.Nil(t, logs.request.Search, "whitespace-only queries must use the ordinary log view")
					require.Equal(t, int64(123), logs.anchorID, "whitespace-only queries must preserve the log anchor")
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
			{"instance", "?instance=api-1", true},
			{"multiple instances", "?instance=api-1&instance=api-2", true},
			{"unnamed instance", "?instance=", true},
			{"range", "?from=2026-11-01&to=2026-11-02&instance=api-1", false},
			{"precise range", "?fromAt=2026-11-01T10:15:00Z&toAt=2026-11-01T10:16:00Z&search=error", false},
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
				require.Equal(t, tc.live, strings.Contains(rec.Body.String(), "hx-ws:connect="), "only the latest view without search, range or anchor may mount a live connection, including selected instances")
				if tc.live {
					connection := regexp.MustCompile(`hx-ws:connect="([^"]+)"`).FindStringSubmatch(rec.Body.String())
					require.Len(t, connection, 2, "a live page must contain one encoded connection URL")
					wsURL, err := url.Parse(html.UnescapeString(connection[1]))
					require.NoError(t, err, "the WebSocket URL must be valid")
					expected, err := url.Parse(tc.query)
					require.NoError(t, err, "the instance query fixture must be valid")
					require.Equal(t, expected.Query()["instance"], wsURL.Query()["instance"], "the WebSocket selection must exactly match the HTTP view, including an empty instance name")
				}
				if requestType == "partial" && (logs.request.Older != nil || logs.request.Newer != nil) {
					require.Zero(t, logs.instanceCalls, "pagination must reuse the mounted view's instance colors without reloading the list")
					require.NotContains(t, rec.Body.String(), "<header", "pagination fragments must not replace the page header")
				} else {
					require.Equal(t, 1, logs.instanceCalls, "every complete log view must load the client's full instance list, regardless of filters")
					seed := regexp.MustCompile(`data-log-instances="([^"]*)"`).FindStringSubmatch(rec.Body.String())
					require.Len(t, seed, 2, "every complete view must provide its instance color seed")
					require.JSONEq(t, `["api-1","api-2","api-3"]`, html.UnescapeString(seed[1]), "the color seed must include instances outside the visible page and selection")
					require.Contains(t, rec.Body.String(), `data-log-client-id="`+clientID+`"`, "the palette seed must identify the authorized client in every log view")
					require.Equal(t, !tc.live, strings.Contains(rec.Body.String(), `aria-label="Back to live"`), "every non-live page must offer a return to the live view")
					status := "Not live"
					if logs.anchorID != 0 {
						status = "Line context"
					}
					require.Equal(t, !tc.live, strings.Contains(rec.Body.String(), status), "the header must reflect the same live state as the connection")
				}
			})
		}
	}
}

func TestLogSearchValidation(t *testing.T) {
	for _, hx := range []bool{false, true} {
		for _, query := range []string{"id", "ab cd", "a-b-c", "%_", "id=1"} {
			t.Run(fmt.Sprintf("%s/htmx=%t", query, hx), func(t *testing.T) {
				e := echo.New()
				logs := &logSearchStub{}
				handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true}, loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
				req := httptest.NewRequest(http.MethodGet, "/logs/"+projectID+"/"+clientID+"?search="+url.QueryEscape(query), nil)
				if hx {
					req.Header.Set("HX-Request", "true")
				}
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				require.Equal(t, http.StatusBadRequest, rec.Code, "the log page must reject queries without an indexable term")
				require.Zero(t, logs.logCalls, "invalid search must not execute a potentially unbounded log query")
				if hx {
					require.Equal(t, "none", rec.Header().Get("HX-Reswap"), "invalid search must preserve the current results")
					require.Contains(t, rec.Header().Get("HX-Trigger"), "three consecutive", "invalid search must explain how to correct the query")
				} else {
					require.Contains(t, rec.Body.String(), "three consecutive", "direct requests must explain the search validation failure")
				}
			})
		}
	}
}

func TestLogCursorValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		query  string
		status int
		micros int64
	}{
		{"microseconds", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7&direction=backward", 200, 1791979200123456},
		{"legacy milliseconds", "cursorTime=1791979200123&cursorSequenceNumber=2&cursorId=7&direction=forward", 400, 0},
		{"forward direction", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7&direction=forward", 200, 1791979200123456},
		{"missing ID", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&direction=backward", 400, 0},
		{"missing direction", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7", 400, 0},
		{"invalid direction", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=7&direction=sideways", 400, 0},
		{"missing sequence", "cursorTimeUs=1791979200123456&cursorId=7&direction=backward", 400, 0},
		{"missing timestamp", "cursorSequenceNumber=2&cursorId=7&direction=backward", 400, 0},
		{"ambiguous timestamp", "cursorTime=123&cursorTimeUs=123000&cursorSequenceNumber=2&cursorId=7&direction=backward", 400, 0},
		{"nonpositive timestamp", "cursorTimeUs=0&cursorSequenceNumber=2&cursorId=7&direction=backward", 400, 0},
		{"invalid timestamp", "cursorTimeUs=invalid&cursorSequenceNumber=2&cursorId=7&direction=backward", 400, 0},
		{"timestamp overflow", "cursorTimeUs=9223372036854775808&cursorSequenceNumber=2&cursorId=7&direction=backward", 400, 0},
		{"invalid id", "cursorTimeUs=1791979200123456&cursorSequenceNumber=2&cursorId=-1&direction=backward", 400, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			logs := &logSearchStub{}
			handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true}, loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
			req := httptest.NewRequest(http.MethodGet, "/logs/"+projectID+"/"+clientID+"?search=connection&"+tc.query, nil)
			req.Header.Set("HX-Request", "true")
			req.Header.Set("HX-Request-Type", "partial")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, tc.status, rec.Code, "the log page must validate cursor parameters")
			if tc.status != http.StatusOK {
				require.Zero(t, logs.logCalls, "invalid cursors must be rejected before querying logs")
				return
			}
			cursor := logs.request.Older
			if strings.Contains(tc.query, "direction=backward") {
				require.Nil(t, logs.request.Newer, "backward navigation uses only the older cursor")
			} else {
				require.Nil(t, logs.request.Older, "forward navigation uses only the newer cursor")
				cursor = logs.request.Newer
			}
			require.NotNil(t, cursor, "a complete cursor must reach the reader")
			require.Equal(t, time.UnixMicro(tc.micros), cursor.OccurredAt, "cursor parsing must preserve microsecond precision")
			require.Equal(t, int64(2), cursor.Sequence, "cursor parsing must preserve the sequence number")
			require.Equal(t, int64(7), cursor.ID, "cursor parsing must preserve the row id tie breaker")
			require.Zero(t, logs.instanceCalls, "pagination fragments must not reload filter choices")
			require.NotContains(t, rec.Body.String(), "<html", "pagination must return only log rows")
			require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search pagination must never connect a WebSocket")
		})
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
			require.Equal(t, []string{"api-1", "api-2"}, logs.request.Instances, "pagination must retain every selected instance")
			require.Equal(t, 1, logs.instanceCalls, "full search pages must load filter choices")
			require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "search pages opened at a cursor must remain non-live")
		})
	}
}

func TestContextPaginationUsesResolvedInstance(t *testing.T) {
	for _, query := range []string{"?logLine=42", "?logLine=42&instance=wrong"} {
		t.Run(query, func(t *testing.T) {
			e := echo.New()
			instance := "api-1"
			logs := &logSearchStub{page: &logstore.Page{
				AnchorInstance: &instance,
				Rows:           []logstore.Row{{ID: 43}, {ID: 42}, {ID: 41}},
				Newer:          &logstore.Cursor{OccurredAt: time.Unix(100, 0), Sequence: 1, ID: 43},
				Older:          &logstore.Cursor{OccurredAt: time.Unix(90, 0), Sequence: 1, ID: 41},
			}}
			handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true}, loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logs/"+projectID+"/"+clientID+query, nil))
			require.Equal(t, http.StatusOK, rec.Code, "old and conflicting context URLs must render successfully")
			links := regexp.MustCompile(`data-log-page="(forward|backward)"[^>]*hx-get="([^"]+)"`).FindAllStringSubmatch(rec.Body.String(), -1)
			require.Len(t, links, 2, "context must offer both pagination directions")
			for _, link := range links {
				req := httptest.NewRequest(http.MethodGet, html.UnescapeString(link[2]), nil)
				req.Header.Set("HX-Request", "true")
				page := httptest.NewRecorder()
				e.ServeHTTP(page, req)
				require.Equal(t, http.StatusOK, page.Code, "each context continuation must load successfully")
				require.Equal(t, []string{"api-1"}, logs.request.Instances, "following either link must use the server-resolved instance, not the original query")
				require.Equal(t, link[1] == "backward", logs.request.Older != nil, "the older link must load older rows")
				require.Equal(t, link[1] == "forward", logs.request.Newer != nil, "the newer link must load newer rows")
				require.NotContains(t, page.Body.String(), "hx-ws:connect=", "neither continuation may mount a live WebSocket")
			}
		})
	}
}

func TestMissingLogContextFallsBackToPage(t *testing.T) {
	for _, anchor := range []string{"999", "not-an-id"} {
		t.Run(anchor, func(t *testing.T) {
			e := echo.New()
			logs := &logSearchStub{}
			handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true}, loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/logs/"+projectID+"/"+clientID+"?logLine="+anchor+"&instance=api-1", nil))
			require.Equal(t, http.StatusOK, rec.Code, "a missing or invalid anchor must render a regular page")
			require.Contains(t, rec.Body.String(), "matching connection refused", "fallback must render the requested client's logs")
			require.Equal(t, []string{"api-1"}, logs.request.Instances, "fallback must keep the instance selection")
			require.NotContains(t, rec.Body.String(), "hx-ws:connect=", "an anchored URL remains historical when the anchor cannot be resolved")
		})
	}
}

func TestLogSearchKeepsPreciseRange(t *testing.T) {
	e := echo.New()
	logs := &logSearchStub{}
	handlers.NewLogsRouter(logs, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true},
		loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
	path := "/logs/" + projectID + "/" + clientID
	query := "?fromAt=2026-11-01T10:15:00Z&toAt=2026-11-01T10:16:00Z&search=error&instance=api-1&instance=api-2"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+query, nil))
	require.Equal(t, http.StatusOK, rec.Code, "the precise range must render successfully")
	require.Equal(t, []string{"error"}, logs.request.Search.Terms, "the precise range must retain the parsed search")
	require.Equal(t, []string{"api-1", "api-2"}, logs.request.Instances, "the precise range must retain both selected instances")
	require.Equal(t, time.Date(2026, 11, 1, 10, 15, 0, 0, time.UTC), logs.request.Range.From, "the lower bound must reach the reader exactly")
	require.Equal(t, time.Date(2026, 11, 1, 10, 16, 0, 0, time.UTC), logs.request.Range.To, "the upper bound must reach the reader exactly")
	require.Contains(t, rec.Body.String(), `name="fromAt" value="2026-11-01T10:15:00Z"`, "the range form must retain its start")
	require.Contains(t, rec.Body.String(), `name="toAt" value="2026-11-01T10:16:00Z"`, "the range form must retain its end")

	for _, invalid := range []string{
		"?fromAt=2026-11-01T10:15:00Z",
		"?fromAt=2026-11-01T10:16:00Z&toAt=2026-11-01T10:15:00Z",
		query + "&from=2026-11-01",
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+invalid, nil))
		require.Equal(t, http.StatusBadRequest, rec.Code, "invalid interval %q must be rejected", invalid)
	}
}
