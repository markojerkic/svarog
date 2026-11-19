package handlers

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/forms"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/pages/admin"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func projectPageError(c *echo.Context, status int, message string) error {
	if htmx.IsHXRequest(c) {
		htmx.AddErrorToast(c, message)
		c.Response().Header().Set("HX-Reswap", "none")
		return c.NoContent(status)
	}
	return utils.Render(c, status, pages.ErrorPage(pages.ErrorPageProps{StatusCode: status, Message: message}))
}

// mutationPageInput reads the current list view before the request can write.
func mutationPageInput(c *echo.Context, path string) (forms.PageInput, error) {
	query := forms.PageInput{Size: 10}
	current := c.Request().Header.Get("HX-Current-URL")
	if !htmx.IsHXRequest(c) || current == "" {
		return query, nil
	}
	view, err := url.Parse(current)
	if err != nil {
		return query, err
	}
	// Use only the query for this page, regardless of the supplied origin.
	if view.Path != path {
		return query, nil
	}
	values, err := url.ParseQuery(view.RawQuery)
	if err != nil {
		return query, err
	}
	query.Search = values.Get("search")
	for name, target := range map[string]*int64{"page": &query.Page, "size": &query.Size} {
		if value := values.Get(name); value != "" {
			*target, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return query, err
			}
		}
	}
	return query, nil
}

func projectMutationQuery(c *echo.Context) (projects.PageQuery, error) {
	input, err := mutationPageInput(c, "/admin/projects")
	if err != nil {
		return projects.PageQuery{}, err
	}
	return projects.NewPageQuery(input.Search, input.Page, input.Size)
}

func (p *ProjectsRouter) renderProjectsPage(c *echo.Context, query projects.PageQuery, mutation bool) error {
	items, count, err := p.projectsService.Page(c.Request().Context(), query)
	if err != nil {
		slog.Error("Error loading project page", "error", err)
		return projectPageError(c, http.StatusInternalServerError, i18n.T(c.Request().Context(), "projects.load_failed"))
	}
	if mutation && query.Page() > 0 {
		lastPage := int64(0)
		if count > 0 {
			lastPage = (count - 1) / query.Size()
		}
		if query.Page() > lastPage {
			query = query.ClampPage(lastPage)
			return p.renderProjectsPage(c, query, true)
		}
	}
	if mutation {
		// Reload the complete view: its rows, order, totals, dialogs and fresh
		// creation form must all describe the same filtered page after a write.
		htmx.Reswap(c, htmx.ReswapProps{Swap: "outerHTML", Target: "body", Select: "body"})
		values := url.Values{}
		if query.Search() != "" {
			values.Set("search", query.Search())
		}
		if query.Page() > 0 {
			values.Set("page", strconv.FormatInt(query.Page(), 10))
		}
		if query.Size() != 10 {
			values.Set("size", strconv.FormatInt(query.Size(), 10))
		}
		view := url.URL{Path: "/admin/projects", RawQuery: values.Encode()}
		c.Response().Header().Set("HX-Replace-Url", view.String())
	}
	return utils.Render(c, http.StatusOK, admin.ProjectsListPage(admin.ProjectsListPageProps{
		Projects: items, Page: query.Page(), Size: query.Size(), Search: query.Search(), TotalCount: count,
	}))
}
