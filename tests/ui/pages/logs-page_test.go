package pages_test

import (
	"bytes"
	"context"
	"html"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/ui/components/logs"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
)

func TestLogColumns(t *testing.T) {
	for _, tc := range []struct {
		name           string
		value          string
		time, instance bool
	}{
		{"missing cookie hides both", "", false, false},
		{"time only", "time", true, false},
		{"instance only", "instance", false, true},
		{"both", "instance,time", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			showTime, showInstance := pages.LogColumns(tc.value)
			if showTime != tc.time || showInstance != tc.instance {
				t.Fatalf("Column cookie %q must show time=%v instance=%v, got time=%v instance=%v", tc.value, tc.time, tc.instance, showTime, showInstance)
			}
		})
	}
}

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
		{"logs", pages.LogsPage(pages.LogsPageProps{ProjectId: "project", ClientId: "client"}), true},
		{"other page", pages.AdminLayout(pages.AdminLayoutProps{Title: "Users"}), false},
		{"pagination", pages.LogPageLines(pages.LogsPageProps{}), false},
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

func TestLogsPageLiveState(t *testing.T) {
	anchor := "42"
	for _, tc := range []struct {
		name       string
		search     string
		instances  *[]string
		logLineId  *string
		isLastPage bool
		live       bool
	}{
		{name: "live", isLastPage: true, live: true},
		{name: "search", search: "connection refused", isLastPage: true},
		{name: "instance", instances: &[]string{"api-1"}, isLastPage: true, live: true},
		{name: "empty instance selection", instances: &[]string{}, isLastPage: true, live: true},
		{name: "filtered search", search: "connection refused", instances: &[]string{"api-1", "api 2"}, isLastPage: true},
		{name: "anchor", logLineId: &anchor, isLastPage: true},
		{name: "pagination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			props := pages.LogsPageProps{
				ProjectId: "project", ClientId: "client", Search: tc.search,
				Instances: tc.instances, LogLineId: tc.logLineId,
				LogPage: db.LogPage{IsLastPage: tc.isLastPage, BackwardCursor: &db.LastCursor{}},
			}
			if err := pages.LogsPage(props).Render(context.Background(), &output); err != nil {
				t.Fatalf("Logs page must render successfully: %v", err)
			}
			rendered := output.String()
			if strings.Contains(rendered, `hx-ws:connect=`) != tc.live {
				t.Error("Only the latest view without search or anchor may mount a live connection, including selected instances")
			}
			for _, fragment := range []string{`data-log-page`, `root:#log-scroll-container`, `hx-sync="body:abort"`, `hx-push-url="false"`, `Load older logs`} {
				if !strings.Contains(rendered, fragment) {
					t.Errorf("Every log mode must render pagination with %s", fragment)
				}
			}
			if strings.Contains(rendered, `No logs found.`) != (tc.search != "") {
				t.Error("Only an empty search must render the no-results message")
			}
			if !strings.Contains(rendered, `data-templ-controls="log-instance-filters"`) {
				t.Error("Instance selection must be available in live and historical views")
			}
			status := "Not live"
			if tc.logLineId != nil {
				status = "Line context"
			}
			for _, fragment := range []string{`aria-label="Back to live"`, status} {
				if strings.Contains(rendered, fragment) != !tc.live {
					t.Errorf("Every non-live view must render %s", fragment)
				}
			}
			if !tc.live {
				link := regexp.MustCompile(`<a[^>]*href="([^"]*)"[^>]*aria-label="Back to live"`).FindStringSubmatch(rendered)
				if len(link) != 2 {
					t.Fatal("Every historical view must provide a link back to live logs")
				}
				u, err := url.Parse(html.UnescapeString(link[1]))
				if err != nil || u.Path != "/logs/project/client" || u.Query().Has("search") || u.Query().Has("logLine") || u.Query().Has("cursorId") {
					t.Fatal("Back to live must retain the client while clearing historical search, anchor and cursor")
				}
				if tc.instances != nil && !slices.Equal(u.Query()["instance"], *tc.instances) {
					t.Error("Back to live must retain the selected instances")
				}
			}
		})
	}
}

func TestClearInstanceFilterReplacesCompleteView(t *testing.T) {
	var output bytes.Buffer
	props := pages.LogsPageProps{ProjectId: "project", ClientId: "client", Instances: &[]string{"api-1"}}
	if err := pages.InstanceFilter(props).Render(context.Background(), &output); err != nil {
		t.Fatalf("Instance filter must render successfully: %v", err)
	}
	for _, fragment := range []string{`href="/logs/project/client"`, `aria-label="Clear instance filter"`, `hx-target="body"`, `hx-swap="innerHTML"`, `hx-sync="body:replace"`} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("Clearing the instance filter must update both the header and logs using %s", fragment)
		}
	}
}

func TestLogInstanceFilters(t *testing.T) {
	instance := "api 01 & backup"
	props := pages.LogsPageProps{
		ProjectId: "project", ClientId: "client", Search: "error & timeout",
		Instances: &[]string{instance}, AvailableInstances: []string{instance, "api-02"},
	}
	var output bytes.Buffer
	if err := pages.LogInstanceFilters(props).Render(context.Background(), &output); err != nil {
		t.Fatalf("Instance filters must render successfully: %v", err)
	}
	links := regexp.MustCompile(`<a\b[^>]*href="([^"]*)"[^>]*\bdata-log-instance-filter\b`).FindAllStringSubmatch(output.String(), -1)
	if len(links) != 4 {
		t.Fatalf("Instance filters must render all instances, both available instances and clear filters: got %d links", len(links))
	}
	wantInstances := map[string]int{"": 2, instance: 1, "api-02": 1}
	for _, link := range links {
		u, err := url.Parse(html.UnescapeString(link[1]))
		if err != nil {
			t.Fatalf("Instance filter links must be valid URLs: %v", err)
		}
		query := u.Query()
		if u.Path != "/logs/project/client" || query.Get("search") != props.Search {
			t.Fatal("Instance changes must preserve the client and the executed query")
		}
		selection := query.Get("instance")
		if selection == "" {
			if query.Has("instance") || len(query) != 1 {
				t.Fatal("All instances must omit the instance parameter and any cursor or anchor")
			}
		} else if len(query["instance"]) != 1 || len(query) != 2 {
			t.Fatal("Selecting an instance must replace the selection without retaining a cursor or anchor")
		}
		wantInstances[selection]--
	}
	for selection, remaining := range wantInstances {
		if remaining != 0 {
			t.Errorf("Instance filter links must preserve each available instance and both clear actions: instance %q has count difference %d", selection, remaining)
		}
	}
	for _, fragment := range []string{
		`aria-label="Filters (1 active)"`, `data-slot="popover-content"`,
		`All instances`, `Clear filters`, `api-02`, `hidden`,
		`hx-target:inherited="body"`, `hx-swap:inherited="innerHTML"`,
	} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("Instance filter popover must contain %s", fragment)
		}
	}
	if strings.Count(output.String(), `aria-current="true"`) != 1 {
		t.Fatal("Only the selected instance must be marked as current")
	}
}

func TestLogRangeFilterOpensOnSelectedMonth(t *testing.T) {
	props := pages.LogsPageProps{
		ProjectId: "project",
		ClientId:  "client",
		LogPage:   db.LogPage{Range: &db.LogRange{FromDate: "2026-10-25", ToDate: "2026-10-28"}},
	}
	var output bytes.Buffer
	if err := pages.LogRangeFilter(props).Render(context.Background(), &output); err != nil {
		t.Fatalf("Date range filter must render successfully: %v", err)
	}
	rendered := output.String()
	if !strings.Contains(rendered, `data-templ-default-month="2026-10-25"`) {
		t.Fatal("The calendar must open on the first day of the selected range")
	}
	if !strings.Contains(rendered, "2026-10-28") {
		t.Fatal("The calendar must keep the selected end date")
	}

	output.Reset()
	props.LogPage.Range = nil
	if err := pages.LogRangeFilter(props).Render(context.Background(), &output); err != nil {
		t.Fatalf("Date range filter must render successfully: %v", err)
	}
	if strings.Contains(output.String(), "data-templ-default-month") {
		t.Fatal("An empty date filter must leave the calendar month unset")
	}
}
