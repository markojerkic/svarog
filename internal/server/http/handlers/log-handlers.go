package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"log/slog"

	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/lib/projects"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/http/middleware"
	"github.com/markojerkic/svarog/internal/server/i18n"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/markojerkic/svarog/internal/server/ui/pages"
	"github.com/markojerkic/svarog/internal/server/ui/utils"
)

type LogLine struct {
	ID             string             `json:"id"`
	Timestamp      int64              `json:"timestamp"`
	Content        string             `json:"content"`
	SequenceNumber int                `json:"sequenceNumber"`
	Client         types.StoredClient `json:"client"`
}

type LogsRouter struct {
	logService   db.LogService
	parentRouter *echo.Group
	api          *echo.Group
}

var DEFAULT_PAGE_SIZE = int64(300)

type LogsByClientBinding struct {
	ProjectId            string    `param:"projectId"`
	ClientId             string    `param:"clientId"`
	CursorTime           *int64    `query:"cursorTime"`
	CursorTimeUs         *int64    `query:"cursorTimeUs"`
	CursorSequenceNumber *int      `query:"cursorSequenceNumber"`
	CursorID             *int64    `query:"cursorId"`
	Direction            *string   `query:"direction"`
	Instances            *[]string `query:"instance"`
	LogLineId            *string   `query:"logLine"`
	Search               string    `query:"search"`
}

func (params LogsByClientBinding) cursor() (*db.LastCursor, error) {
	if params.CursorTime == nil && params.CursorTimeUs == nil && params.CursorSequenceNumber == nil && params.CursorID == nil && params.Direction == nil {
		return nil, nil
	}
	if params.CursorTime == nil && params.CursorTimeUs == nil || params.CursorTime != nil && params.CursorTimeUs != nil {
		return nil, fmt.Errorf("cursor requires exactly one timestamp")
	}
	if params.CursorSequenceNumber == nil || params.Direction == nil || (*params.Direction != "backward" && *params.Direction != "forward") {
		return nil, fmt.Errorf("cursor requires a sequence number and a valid direction")
	}
	var timestamp time.Time
	if params.CursorTimeUs != nil {
		if *params.CursorTimeUs <= 0 {
			return nil, fmt.Errorf("cursor timestamp must be positive")
		}
		timestamp = time.UnixMicro(*params.CursorTimeUs)
	} else {
		if *params.CursorTime <= 0 || *params.CursorTime > math.MaxInt64/1000 {
			return nil, fmt.Errorf("invalid cursor timestamp")
		}
		timestamp = time.UnixMilli(*params.CursorTime)
	}
	cursor := &db.LastCursor{
		Timestamp:      timestamp,
		SequenceNumber: *params.CursorSequenceNumber,
		IsBackward:     *params.Direction == "backward",
	}
	if params.CursorID != nil {
		if *params.CursorID <= 0 {
			return nil, fmt.Errorf("cursor id must be positive")
		}
		cursor.ID = *params.CursorID
	}
	return cursor, nil
}

func (self *LogsRouter) instancesByClientHandler(c *echo.Context) error {
	projectId := c.Param("projectId")
	clientId := c.Param("clientId")
	if projectId == "" || clientId == "" {
		return c.JSON(400, i18n.T(c.Request().Context(), "logs.ids_required"))
	}
	slog.Debug("Getting instances by client", "projectId", projectId, "clientId", clientId)

	instances, err := self.logService.GetInstances(c.Request().Context(), projectId, clientId)
	if err != nil {
		return err
	}

	return c.JSON(200, instances)
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
	if params.Search != "" {
		params.LogLineId = nil
	}

	nextCursor, err := params.cursor()
	if err != nil {
		return c.JSON(http.StatusBadRequest, i18n.T(c.Request().Context(), "logs.invalid_cursor"))
	}

	logPage, err := self.logService.GetLogs(c.Request().Context(), db.LogPageRequest{
		ProjectId: params.ProjectId,
		ClientId:  params.ClientId,
		Instances: params.Instances,
		PageSize:  DEFAULT_PAGE_SIZE,
		LogLineId: params.LogLineId,
		Cursor:    nextCursor,
		Search:    params.Search,
	})

	if err != nil {
		return err
	}

	if logPage.Instances != nil {
		params.Instances = logPage.Instances
	}
	props := pages.LogsPageProps{
		LogPage:   logPage,
		ClientId:  params.ClientId,
		ProjectId: params.ProjectId,
		Search:    params.Search,
		Instances: params.Instances,
		LogLineId: params.LogLineId,
	}

	isHx := c.Request().Header.Get("HX-Request") == "true"
	if isHx && nextCursor != nil && c.Request().Header.Get("HX-Request-Type") != "full" {
		return utils.Render(c, http.StatusOK, pages.LogPageLines(props))
	}
	props.AvailableInstances, err = self.logService.GetInstances(c.Request().Context(), params.ProjectId, params.ClientId)
	if err != nil {
		return err
	}

	return utils.Render(c, http.StatusOK, pages.LogsPage(props))
}

func (self *LogsRouter) searchLogs(c *echo.Context) error {
	var params LogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		slog.Error("Bindings for logs by client not correct", "error", err)
		return c.JSON(400, i18n.T(c.Request().Context(), "errors.400_title"))
	}

	nextCursor, err := params.cursor()
	if err != nil {
		return c.JSON(http.StatusBadRequest, i18n.T(c.Request().Context(), "logs.invalid_cursor"))
	}

	slog.Debug("next", "cursor", nextCursor)
	logs, err := self.logService.SearchLogs(c.Request().Context(), params.Search, params.ProjectId, params.ClientId, params.Instances, DEFAULT_PAGE_SIZE, nextCursor)

	if err != nil {
		return err
	}

	logsLen := len(logs)
	mappedLogs := make([]LogLine, logsLen)
	for i, log := range logs {
		mappedLogs[logsLen-i-1] = LogLine{
			strconv.FormatInt(log.ID, 10),
			log.Timestamp.UnixMilli(),
			log.LogLine,
			log.SequenceNumber,
			log.Client,
		}
	}

	return c.JSON(200, mappedLogs)
}

func NewLogsRouter(logService db.LogService, authService auth.AuthService, projectsService projects.ProjectsService, e *echo.Group) *LogsRouter {
	logsApi := e.Group("/logs/:projectId/:clientId", middleware.RequiresProjectAccessMiddleware(authService, projectsService))

	logsRouter := &LogsRouter{
		logService:   logService,
		parentRouter: e,
		api:          logsApi,
	}

	logsRouter.api.GET("", logsRouter.logsByClientHandler)
	logsRouter.api.GET("/instances", logsRouter.instancesByClientHandler)
	logsRouter.api.GET("/search", logsRouter.searchLogs)

	return logsRouter
}
