package pages

import (
	"bytes"
	"context"
	"html"
	"net/url"
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
				if strings.Count(html, "/assets/js/log-menu.js") != 1 {
					t.Fatal("The delegated log menu script must be loaded once by the base layout")
				}
				for _, fragment := range []string{`id="log-context-link"`, `Open context`, `id="filter-instance-link"`, `hx-target:inherited="body"`, `hx-swap:inherited="innerHTML"`} {
					if !strings.Contains(html, fragment) {
						t.Errorf("Log menu actions must render %s", fragment)
					}
				}
				if strings.Contains(html, `id="filter-instance-button"`) {
					t.Fatal("Instance filtering must use a navigation link rather than the old hidden button")
				}
				headerEnd := strings.Index(html, "</header>")
				trigger := strings.Index(html, `id="log-search-trigger"`)
				if headerEnd < 0 || trigger > headerEnd {
					t.Fatal("Search trigger must render in the existing header, not a new toolbar")
				}
			}
		})
	}
}

func TestSearchResultsPaginateWithoutLiveUpdates(t *testing.T) {
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
			if strings.Contains(html, `hx-ws:connect=`) != (search == "") {
				t.Error("Only ordinary logs may mount a live connection")
			}
			for _, fragment := range []string{`data-log-page`, `root:#log-scroll-container`, `hx-sync="body:abort"`, `hx-push-url="false"`, `Load older logs`} {
				if !strings.Contains(html, fragment) {
					t.Errorf("Both log modes must render pagination with %s", fragment)
				}
			}
			for _, fragment := range []string{`No logs found.`, `aria-label="Back to live"`, `Not live`, `data-templ-controls="log-instance-filters"`} {
				if strings.Contains(html, fragment) != (search != "") {
					t.Errorf("Only search results must render %s", fragment)
				}
			}
		})
	}
}

func TestLogInstanceFilters(t *testing.T) {
	instance := "api 01 & backup"
	props := LogsPageProps{
		ProjectId: "project", ClientId: "client", Search: "error & timeout",
		Instances: &[]string{instance}, AvailableInstances: []string{instance, "api-02"},
	}
	for _, selection := range []*string{nil, &instance} {
		path := logInstanceFilterPath(props, selection)
		u, err := url.Parse(path)
		if err != nil {
			t.Fatalf("Instance filter links must be valid URLs: %v", err)
		}
		query := u.Query()
		if u.Path != "/logs/project/client" || query.Get("search") != props.Search {
			t.Fatal("Instance changes must preserve the client and the executed query")
		}
		if selection == nil {
			if query.Has("instance") || len(query) != 1 {
				t.Fatal("All instances must omit the instance parameter and any cursor or anchor")
			}
		} else if query.Get("instance") != instance || len(query) != 2 {
			t.Fatal("Selecting an instance must replace the selection without retaining a cursor or anchor")
		}
	}
	var output bytes.Buffer
	if err := LogInstanceFilters(props).Render(context.Background(), &output); err != nil {
		t.Fatalf("Instance filters must render successfully: %v", err)
	}
	for _, fragment := range []string{
		`aria-label="Filters (1 active)"`, `data-slot="popover-content"`,
		`All instances`, `Clear filters`, `api-02`, `hidden`,
		`hx-target:inherited="body"`, `hx-swap:inherited="innerHTML"`,
		`href="` + html.EscapeString(logInstanceFilterPath(props, &instance)) + `"`,
	} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("Instance filter popover must contain %s", fragment)
		}
	}
	if strings.Count(output.String(), `aria-current="true"`) != 1 {
		t.Fatal("Only the selected instance must be marked as current")
	}
}
