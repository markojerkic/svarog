package pages

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
)

func TestLogSearchBrowserFixtureMatchesComponents(t *testing.T) {
	var output bytes.Buffer
	for _, component := range []templ.Component{logs.SearchTrigger(), logs.SearchDialog()} {
		if err := component.Render(context.Background(), &output); err != nil {
			t.Fatalf("Search controls must render successfully: %v", err)
		}
	}
	fixture, err := os.ReadFile("../../../../tests/fixtures/log-search.html")
	if err != nil {
		t.Fatalf("Browser test fixture must be readable: %v", err)
	}
	if string(fixture) != output.String() {
		t.Fatal("Browser test fixture must match the rendered search controls")
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
