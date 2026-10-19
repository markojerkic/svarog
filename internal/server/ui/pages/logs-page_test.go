package pages

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

func TestLogSearchForm(t *testing.T) {
	var output bytes.Buffer
	instances := []string{"api-1", "api-2"}
	if err := logs.SearchDialog("/logs/project/client", "error & timeout", &instances).Render(context.Background(), &output); err != nil {
		t.Fatalf("Search form must render successfully: %v", err)
	}
	for _, fragment := range []string{
		`method="get"`, `action="/logs/project/client"`, `hx-target="body"`, `hx-push-url="true"`,
		`name="search"`, `value="error &amp; timeout"`, `type="submit"`, `aria-label="Run search"`,
		`name="instance" value="api-1"`, `name="instance" value="api-2"`,
	} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("Search form must contain %s", fragment)
		}
	}
	if regexp.MustCompile(`<button[^>]*\sdisabled(?:\s|=|>)`).MatchString(output.String()) {
		t.Fatal("Search submit must be enabled")
	}
}

func TestLogSearchIsOnlyInTheLogsPageHeader(t *testing.T) {
	for _, tc := range []struct {
		name   string
		page   templ.Component
		search bool
	}{
		{"logs", LogsPage(LogsPageProps{ProjectId: "project", ClientId: "client"}), true},
		{"other page", AdminLayout(AdminLayoutProps{Title: "Users"}), false},
		{"pagination", LogPageLines(LogsPageProps{}), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := tc.page.Render(context.Background(), &output); err != nil {
				t.Fatalf("Page must render successfully: %v", err)
			}
			html := output.String()
			if strings.Contains(html, `id="log-search-trigger"`) != tc.search {
				t.Fatal("Search trigger must only appear on the full logs page")
			}
			if tc.search {
				headerEnd := strings.Index(html, "</header>")
				trigger := strings.Index(html, `id="log-search-trigger"`)
				if headerEnd < 0 || trigger > headerEnd {
					t.Fatal("Search trigger must render in the existing header, not a new toolbar")
				}
			}
		})
	}
}

func TestSearchResultsNeverMountLiveOrPagination(t *testing.T) {
	for _, search := range []string{"", "connection refused"} {
		t.Run("search="+search, func(t *testing.T) {
			var output bytes.Buffer
			props := LogsPageProps{
				ProjectId: "project", ClientId: "client", Search: search,
				LogPage: db.LogPage{IsLastPage: true, BackwardCursor: &db.LastCursor{}},
			}
			if err := LogsPage(props).Render(context.Background(), &output); err != nil {
				t.Fatalf("Logs page must render successfully: %v", err)
			}
			html := output.String()
			for _, fragment := range []string{`hx-ws:connect=`, `hx-trigger="intersect once"`} {
				if strings.Contains(html, fragment) != (search == "") {
					t.Errorf("Only ordinary logs may render %s in this first-page search implementation", fragment)
				}
			}
			for _, fragment := range []string{`No logs found.`, `aria-label="Back to live"`, `Not live`} {
				if strings.Contains(html, fragment) != (search != "") {
					t.Errorf("Only search results must render %s", fragment)
				}
			}
		})
	}
}
