package mcpserver_test

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/markojerkic/svarog/internal/lib/archive"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/oauth"
	"github.com/markojerkic/svarog/internal/server/mcpserver"
)

type archiveSearchStub struct {
	mcpserver.Projects
	mcpserver.Archives
	user        auth.User
	scope       archive.Scope
	denyProject bool
	search      func(archive.Scope, archive.SearchQuery) (archive.SearchResult, error)
}

func (s archiveSearchStub) VerifyAccessToken(context.Context, string) (oauth.AccessToken, error) {
	return oauth.AccessToken{UserID: s.user.ID, Scopes: []string{oauth.ScopeLogsRead}, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

func (s archiveSearchStub) GetUserByID(context.Context, string) (auth.User, error) {
	return s.user, nil
}

func (s archiveSearchStub) UserHasAccessToProject(_ context.Context, userID, projectID string) (bool, error) {
	return !s.denyProject && userID == s.user.ID.String() && projectID == s.scope.ProjectID.String(), nil
}

func (s archiveSearchStub) ClientBelongsToProject(_ context.Context, projectID, clientID string) (bool, error) {
	return projectID == s.scope.ProjectID.String() && clientID == s.scope.ClientID.String(), nil
}

func (s archiveSearchStub) Search(_ context.Context, scope archive.Scope, query archive.SearchQuery) (archive.SearchResult, error) {
	return s.search(scope, query)
}

func archiveSearchCaller(t *testing.T, s archiveSearchStub) func(map[string]any) *mcp.CallToolResult {
	t.Helper()
	handler := mcpserver.NewHandler(mcpserver.Options{Tokens: s, Users: s, Projects: s, Archives: s})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer test")
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	session, err := client.Connect(t.Context(), &mcp.StreamableClientTransport{Endpoint: server.URL, DisableStandaloneSSE: true}, nil)
	require.NoError(t, err)
	t.Cleanup(func() { session.Close() })
	return func(args map[string]any) *mcp.CallToolResult {
		t.Helper()
		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "search_archived_logs", Arguments: args})
		require.NoError(t, err)
		return result
	}
}

func TestSearchArchivedLogs(t *testing.T) {
	scope := archive.Scope{ProjectID: uuid.New(), ClientID: uuid.New()}
	from := time.Date(2026, 1, 1, 12, 0, 0, 123456789, time.FixedZone("UTC+1", 3600))
	to := from.Add(time.Hour)
	id := uuid.New()
	long := strings.Repeat("č", 2000)
	call := archiveSearchCaller(t, archiveSearchStub{
		user: auth.User{ID: uuid.New(), Role: auth.ADMIN}, scope: scope,
		search: func(got archive.Scope, q archive.SearchQuery) (archive.SearchResult, error) {
			if got != scope || !q.From.Equal(from) || !q.To.Equal(to) {
				t.Errorf("unexpected archive search scope %v or time range %v", got, q)
			}
			switch q.Text {
			case "":
				return archive.SearchResult{}, nil
			case "too broad":
				return archive.SearchResult{}, archive.ErrSearchLimit
			case "timeout retry":
				hits := make([]archive.SearchHit, 200)
				for i := range hits {
					hits[i] = archive.SearchHit{ArchiveID: id, Entry: "web/logs_1.log", Line: 201 - i,
						OccurredAt: from.Add(time.Duration(200-i) * time.Second), Instance: "web", Message: "timeout retry"}
				}
				hits[0].Message = long + " timeout retry"
				return archive.SearchResult{ArchivesScanned: 2, TotalMatches: 205, Hits: hits}, nil
			default:
				t.Errorf("unexpected query: %q", q.Text)
				return archive.SearchResult{}, nil
			}
		},
	})
	args := map[string]any{"project_id": scope.ProjectID, "client_id": scope.ClientID, "from": from, "to": to}
	result := call(args)
	require.False(t, result.IsError, "%v", result.Content)
	require.Equal(t, map[string]any{"archives_scanned": float64(0), "total_matches": float64(0), "rows": []any{}}, result.StructuredContent)

	args["query"] = "timeout retry"
	result = call(args)
	require.False(t, result.IsError, "%v", result.Content)
	out := result.StructuredContent.(map[string]any)
	require.NotContains(t, out, "older_cursor")
	require.NotContains(t, out, "newer_cursor")
	require.Equal(t, float64(2), out["archives_scanned"])
	require.Equal(t, float64(205), out["total_matches"])
	require.Equal(t, true, out["truncated"])
	rows := out["rows"].([]any)
	require.Len(t, rows, 200)
	require.Equal(t, "timeout retry", rows[0].(map[string]any)["message"])
	require.Equal(t, map[string]any{
		"archive_id": id.String(), "entry": "web/logs_1.log", "line": float64(201),
		"occurred_at": from.UTC().Add(200 * time.Second).Format(time.RFC3339Nano), "instance": "web",
		"message": long, "truncated": true,
	}, rows[199])

	args["query"] = "too broad"
	result = call(args)
	require.True(t, result.IsError)
	require.Nil(t, result.StructuredContent)
	require.Contains(t, result.Content[0].(*mcp.TextContent).Text, "narrower time range")
}

func TestSearchArchivedLogsRejectsBeforeScan(t *testing.T) {
	s := archiveSearchStub{
		user: auth.User{ID: uuid.New(), Role: auth.ADMIN}, scope: archive.Scope{ProjectID: uuid.New(), ClientID: uuid.New()},
		search: func(archive.Scope, archive.SearchQuery) (archive.SearchResult, error) {
			t.Error("denied or invalid requests must never scan archives")
			return archive.SearchResult{}, nil
		},
	}
	args := map[string]any{
		"project_id": s.scope.ProjectID, "client_id": s.scope.ClientID, "from": "2026-01-01T00:00:00Z", "to": "2026-01-02T00:00:00Z",
	}
	adminCall := archiveSearchCaller(t, s)
	for _, tc := range []struct {
		name, field string
		value       any // nil omits the field
	}{
		{"client outside project", "client_id", uuid.New()},
		{"missing project", "project_id", nil},
		{"missing client", "client_id", nil},
		{"missing start", "from", nil},
		{"missing end", "to", nil},
		{"invalid timestamp", "from", "2026-01-01"},
		{"empty range", "to", args["from"]},
		{"reversed range", "to", "2025-01-01T00:00:00Z"},
		{"oversized query", "query", strings.Repeat("č", archive.MaxSearchText+1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := maps.Clone(args)
			if tc.value == nil {
				delete(input, tc.field)
			} else {
				input[tc.field] = tc.value
			}
			require.True(t, adminCall(input).IsError)
		})
	}
	t.Run("admin without project access", func(t *testing.T) {
		denied := s
		denied.denyProject = true
		require.True(t, archiveSearchCaller(t, denied)(args).IsError)
	})
	t.Run("non-admin with project access", func(t *testing.T) {
		denied := s
		denied.user.Role = auth.USER
		result := archiveSearchCaller(t, denied)(args)
		require.True(t, result.IsError)
		require.Contains(t, result.Content[0].(*mcp.TextContent).Text, "only to administrators")
	})
}
