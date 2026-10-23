package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/components/combobox"
	usercomponents "github.com/markojerkic/svarog/internal/server/ui/components/users"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/pages/admin"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

func userPageError(c *echo.Context, status int, message string) error {
	if htmx.IsHXRequest(c) {
		htmx.AddErrorToast(c, message)
		c.Response().Header().Set("HX-Reswap", "none")
		return c.NoContent(status)
	}
	return utils.Render(c, status, pages.ErrorPage(pages.ErrorPageProps{StatusCode: status, Message: message}))
}

func userMutationQuery(c *echo.Context) (types.GetUserPageInput, error) {
	query := types.GetUserPageInput{Size: 10}
	current := c.Request().Header.Get("HX-Current-URL")
	if !htmx.IsHXRequest(c) || current == "" {
		return query, nil
	}
	view, err := url.Parse(current)
	if err != nil {
		return query, types.NewApiError(i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	if view.Path != "/admin/users" {
		return query, nil
	}
	values, err := url.ParseQuery(view.RawQuery)
	if err != nil {
		return query, types.NewApiError(i18n.T(c.Request().Context(), "request.invalid_parameters"))
	}
	query.Search = values.Get("search")
	for name, target := range map[string]*int64{"page": &query.Page, "size": &query.Size} {
		if value := values.Get(name); value != "" {
			*target, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return query, types.NewApiError(i18n.T(c.Request().Context(), "request.invalid_parameters"))
			}
		}
	}
	return query, auth.ValidateUserPage(c.Request().Context(), query)
}

func (a *AuthRouter) renderUsersPage(c *echo.Context, query types.GetUserPageInput, mutation bool) error {
	users, count, err := a.authService.GetUserPage(c.Request().Context(), query)
	if err != nil {
		var apiErr types.ApiError
		if errors.As(err, &apiErr) {
			return userPageError(c, http.StatusBadRequest, apiErr.Message)
		}
		slog.Error("Error loading users page", "error", err)
		return userPageError(c, http.StatusInternalServerError, i18n.T(c.Request().Context(), "users.load_failed"))
	}
	if mutation && query.Page > 0 {
		lastPage := int64(0)
		if count > 0 {
			lastPage = (count - 1) / query.Size
		}
		if query.Page > lastPage {
			query.Page = lastPage
			return a.renderUsersPage(c, query, true)
		}
	}
	if mutation {
		htmx.Reswap(c, htmx.ReswapProps{Swap: "outerHTML", Target: "body", Select: "body"})
		values := url.Values{}
		if query.Search != "" {
			values.Set("search", query.Search)
		}
		if query.Page > 0 {
			values.Set("page", strconv.FormatInt(query.Page, 10))
		}
		if query.Size != 10 {
			values.Set("size", strconv.FormatInt(query.Size, 10))
		}
		view := url.URL{Path: "/admin/users", RawQuery: values.Encode()}
		c.Response().Header().Set("HX-Replace-Url", view.String())
	}
	return utils.Render(c, http.StatusOK, admin.UsersListPage(admin.UsersListPageProps{
		Users: users, Page: query.Page, Size: query.Size, Search: query.Search, TotalCount: count,
	}))
}

func (a *AuthRouter) userProjectOptions(c *echo.Context, ids types.CommaSeparatedStrings) ([]combobox.Option, error) {
	options := make([]combobox.Option, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	var validIDs []string
	for _, value := range ids {
		if seen[value] {
			continue
		}
		seen[value] = true
		options = append(options, combobox.Option{Value: value, Name: value})
		id, err := uuid.Parse(value)
		if err == nil && id != uuid.Nil && id.String() == value {
			validIDs = append(validIDs, value)
		}
	}
	if len(validIDs) == 0 || a.projectsService == nil {
		return options, nil
	}
	projects, err := a.projectsService.GetProjectsByIds(c.Request().Context(), validIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(projects))
	for _, project := range projects {
		names[project.ID.String()] = project.Name
	}
	for index := range options {
		if name, found := names[options[index].Value]; found {
			options[index].Name = name
		}
	}
	return options, nil
}

func (a *AuthRouter) renderUserFormError(c *echo.Context, status int, form types.CreateUserForm, apiErr types.ApiError) error {
	options, err := a.userProjectOptions(c, form.ProjectIDs)
	if err != nil {
		slog.Error("Error loading submitted user projects", "error", err)
		// A failed lookup must not replace named chips with raw IDs or lose the
		// current form state. Keep the browser's form mounted and allow a retry.
		return userPageError(c, http.StatusInternalServerError, i18n.T(c.Request().Context(), "projects.selected_load_failed"))
	}
	form.Projects = options
	htmx.Reswap(c, htmx.ReswapProps{Swap: "outerHTML", Target: "this", Select: "form"})
	return utils.Render(c, status, usercomponents.NewUserForm(usercomponents.NewUserFormProps{ApiError: apiErr, Value: form}))
}
