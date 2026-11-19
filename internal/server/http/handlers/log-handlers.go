package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/logsearch"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/http/htmx"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type LogReader interface {
	Page(ctx context.Context, query logstore.PageQuery) (logstore.Page, error)
	Around(ctx context.Context, scope logstore.Scope, id int64, limit int, bounds *logstore.TimeRange) (logstore.Page, bool, error)
	Instances(ctx context.Context, scope logstore.Scope) ([]string, error)
}

type LogsRouter struct {
	logReader       LogReader
	projectsService projects.ProjectsService
}

const DEFAULT_PAGE_SIZE = 300

type LogsByClientBinding struct {
	ProjectId            string    `param:"projectId"`
	ClientId             string    `param:"clientId"`
	LegacyCursorTime     *string   `query:"cursorTime"`
	CursorTimeUs         *int64    `query:"cursorTimeUs"`
	CursorSequenceNumber *int64    `query:"cursorSequenceNumber"`
	CursorID             *int64    `query:"cursorId"`
	Direction            *string   `query:"direction"`
	Instances            *[]string `query:"instance"`
	LogLineId            *string   `query:"logLine"`
	Search               string    `query:"search"`
	From                 string    `query:"from"`
	To                   string    `query:"to"`
	Offset               *int      `query:"offset"`
	FromAt               string    `query:"fromAt"`
	ToAt                 string    `query:"toAt"`
	ReturnSearch         string    `query:"returnSearch"`
	ReturnInstances      []string  `query:"returnInstance"`
}

func (params LogsByClientBinding) cursor() (*logstore.Cursor, error) {
	if params.LegacyCursorTime != nil {
		return nil, fmt.Errorf("cursor requires a microsecond timestamp")
	}
	if params.CursorTimeUs == nil && params.CursorSequenceNumber == nil && params.CursorID == nil && params.Direction == nil {
		return nil, nil
	}
	if params.CursorTimeUs == nil || *params.CursorTimeUs <= 0 || params.CursorSequenceNumber == nil ||
		params.CursorID == nil || *params.CursorID <= 0 || params.Direction == nil ||
		(*params.Direction != "backward" && *params.Direction != "forward") {
		return nil, fmt.Errorf("cursor requires a positive timestamp and id, a sequence number, and a valid direction")
	}
	return &logstore.Cursor{
		OccurredAt: time.UnixMicro(*params.CursorTimeUs), Sequence: *params.CursorSequenceNumber, ID: *params.CursorID,
	}, nil
}

func logScope(project, client string) logstore.Scope {
	projectID, _ := uuid.Parse(project)
	clientID, _ := uuid.Parse(client)
	return logstore.Scope{ProjectID: projectID, ClientID: clientID}
}

func (params LogsByClientBinding) pageQuery(cursor *logstore.Cursor, search *logsearch.Query, bounds *logstore.TimeRange) logstore.PageQuery {
	query := logstore.PageQuery{
		Scope: logScope(params.ProjectId, params.ClientId), Search: search, Range: bounds, Limit: DEFAULT_PAGE_SIZE,
	}
	if params.Instances != nil {
		query.Instances = *params.Instances
	}
	if cursor != nil {
		if *params.Direction == "backward" {
			query.Older = cursor
		} else {
			query.Newer = cursor
		}
	}
	return query
}

func (params LogsByClientBinding) logRange() (*pages.LogRange, error) {
	if params.FromAt != "" || params.ToAt != "" {
		if params.From != "" || params.To != "" || params.Offset != nil {
			return nil, fmt.Errorf("cannot combine date and time ranges")
		}
		parsed, err := ParseLogTimeRange(params.FromAt, params.ToAt)
		return &parsed, err
	}
	if params.From == "" && params.To == "" {
		return nil, nil
	}
	from, to := params.From, params.To
	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	offset := 0
	if params.Offset != nil {
		offset = *params.Offset
	}
	parsed, err := ParseLogRange(from, to, offset)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// ParseLogTimeRange accepts an absolute, half-open interval.
func ParseLogTimeRange(fromAt, toAt string) (pages.LogRange, error) {
	from, fromErr := time.Parse(time.RFC3339Nano, fromAt)
	to, toErr := time.Parse(time.RFC3339Nano, toAt)
	if fromErr != nil || toErr != nil || !from.Before(to) {
		return pages.LogRange{}, fmt.Errorf("invalid log time range")
	}
	return pages.LogRange{TimeRange: logstore.TimeRange{From: from, To: to}}, nil
}

// ParseLogRange turns inclusive local calendar dates into a half-open UTC
// interval. offsetMin is JavaScript's getTimezoneOffset: minutes to add to
// local time to get UTC.
func ParseLogRange(fromDate, toDate string, offsetMin int) (pages.LogRange, error) {
	if offsetMin < -14*60 || offsetMin > 14*60 {
		return pages.LogRange{}, fmt.Errorf("invalid timezone offset")
	}
	fromDay, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return pages.LogRange{}, fmt.Errorf("invalid from date")
	}
	toDay, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return pages.LogRange{}, fmt.Errorf("invalid to date")
	}
	if toDay.Before(fromDay) {
		fromDay, toDay = toDay, fromDay
		fromDate, toDate = toDate, fromDate
	}
	offset := time.Duration(offsetMin) * time.Minute
	return pages.LogRange{
		TimeRange: logstore.TimeRange{From: fromDay.Add(offset), To: toDay.AddDate(0, 0, 1).Add(offset)},
		FromDate:  fromDate,
		ToDate:    toDate,
		Offset:    offsetMin,
	}, nil
}

func (self *LogsRouter) logClientName(ctx context.Context, projectID, clientID string) string {
	if self.projectsService == nil {
		return ""
	}
	project, err := self.projectsService.GetProject(ctx, projectID)
	if err != nil {
		slog.Debug("log page client name unavailable", "error", err)
		return ""
	}
	for _, client := range project.Clients {
		if client.ID.String() == clientID {
			return client.Name
		}
	}
	return ""
}

func (self *LogsRouter) logsByClientHandler(c *echo.Context) error {
	var params LogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		slog.Error("Bindings for logs by client not correct", "error", err)
		return c.JSON(400, i18n.T(c.Request().Context(), "errors.400_title"))
	}

	slog.Debug("Get logs by client", "params", params)
	params.Search = strings.TrimSpace(params.Search)
	if params.Instances != nil {
		selector, err := live.NewSelector(*params.Instances)
		if err != nil {
			return c.JSON(http.StatusBadRequest, i18n.T(c.Request().Context(), "logs.invalid_instances"))
		}
		params.Instances = &selector.Instances
		if len(selector.Instances) == 0 {
			params.Instances = nil
		}
	}
	parsedSearch, err := logsearch.Parse(params.Search)
	if err != nil {
		return logSearchValidationError(c)
	}
	if params.Search != "" {
		params.LogLineId = nil
	}

	nextCursor, err := params.cursor()
	if err != nil {
		return c.JSON(http.StatusBadRequest, i18n.T(c.Request().Context(), "logs.invalid_cursor"))
	}
	logRange, err := params.logRange()
	if err != nil {
		return c.JSON(http.StatusBadRequest, i18n.T(c.Request().Context(), "logs.invalid_range"))
	}

	var bounds *logstore.TimeRange
	if logRange != nil {
		bounds = &logRange.TimeRange
	}
	var search *logsearch.Query
	if params.Search != "" {
		search = &parsedSearch
	}
	query := params.pageQuery(nextCursor, search, bounds)
	ctx := c.Request().Context()
	logPage, isLastPage, err := self.readPage(ctx, query, params.LogLineId)
	if err != nil {
		return err
	}
	if logPage.AnchorInstance != nil {
		params.Instances = &[]string{*logPage.AnchorInstance}
	}
	showTime, showInstance := true, true
	cookie, cookieErr := c.Cookie(pages.LogColumnsCookie)
	if cookieErr == nil {
		showTime, showInstance = pages.LogColumns(cookie.Value)
	}
	props := pages.LogsPageProps{
		LogPage:      logPage,
		IsLastPage:   isLastPage,
		Range:        logRange,
		ClientId:     params.ClientId,
		ProjectId:    params.ProjectId,
		Search:       params.Search,
		ShowTime:     showTime,
		ShowInstance: showInstance,
		Instances:    params.Instances,
		LogLineId:    params.LogLineId,
	}

	if params.LogLineId != nil && strings.TrimSpace(params.ReturnSearch) != "" {
		selection, selectionErr := live.NewSelector(params.ReturnInstances)
		if _, searchErr := logsearch.Parse(params.ReturnSearch); searchErr == nil && selectionErr == nil {
			props.ReturnSearch = strings.TrimSpace(params.ReturnSearch)
			props.ReturnInstances = &selection.Instances
		}
	}
	isHx := c.Request().Header.Get("HX-Request") == "true"
	if isHx && nextCursor != nil && c.Request().Header.Get("HX-Request-Type") != "full" {
		return utils.Render(c, http.StatusOK, pages.LogPageLines(props))
	}
	props.ClientName = self.logClientName(c.Request().Context(), params.ProjectId, params.ClientId)
	props.AvailableInstances, err = self.logReader.Instances(ctx, query.Scope)
	if err != nil {
		return err
	}

	return utils.Render(c, http.StatusOK, pages.LogsPage(props))
}

func (self *LogsRouter) readPage(ctx context.Context, query logstore.PageQuery, logLineID *string) (logstore.Page, bool, error) {
	latest := query.Older == nil && query.Newer == nil
	if logLineID != nil {
		id, parseErr := strconv.ParseInt(*logLineID, 10, 64)
		if parseErr == nil && id > 0 {
			if latest {
				page, found, err := self.logReader.Around(ctx, query.Scope, id, query.Limit, query.Range)
				if err != nil {
					return logstore.Page{}, false, err
				}
				if found {
					return page, page.Rows[0].ID == id, nil
				}
			} else {
				query.AnchorID = id
			}
		}
	}
	page, err := self.logReader.Page(ctx, query)
	if err == nil && logLineID != nil && page.AnchorInstance == nil {
		slog.Warn("Log context anchor not found", "logLineId", *logLineID)
	}
	return page, latest, err
}

func logSearchValidationError(c *echo.Context) error {
	message := i18n.T(c.Request().Context(), "logs.search_too_short")
	if htmx.IsHXRequest(c) {
		htmx.AddErrorToast(c, message)
		c.Response().Header().Set("HX-Reswap", "none")
		return c.NoContent(http.StatusBadRequest)
	}
	return utils.Render(c, http.StatusBadRequest, pages.ErrorPage(pages.ErrorPageProps{
		StatusCode: http.StatusBadRequest,
		Message:    message,
	}))
}

func NewLogsRouter(logReader LogReader, authService auth.AuthService, projectsService projects.ProjectsService, e *echo.Group) *LogsRouter {
	logsApi := e.Group("/logs/:projectId/:clientId", middleware.RequiresProjectAccessMiddleware(authService, projectsService))

	logsRouter := &LogsRouter{
		logReader:       logReader,
		projectsService: projectsService,
	}

	logsApi.GET("", logsRouter.logsByClientHandler)

	return logsRouter
}
