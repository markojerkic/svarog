package handlers

import (
	"net/http"
	"time"

	"log/slog"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/db"
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
	CursorSequenceNumber *int      `query:"cursorSequenceNumber"`
	Direction            *string   `query:"direction"`
	Instances            *[]string `query:"instance"`
	LogLineId            *string   `query:"logLine"`
}

func (self *LogsRouter) instancesByClientHandler(c echo.Context) error {
	projectId := c.Param("projectId")
	clientId := c.Param("clientId")
	if projectId == "" || clientId == "" {
		return c.JSON(400, "No project id or client id")
	}
	slog.Debug("Getting instances by client", "projectId", projectId, "clientId", clientId)

	instances, err := self.logService.GetInstances(c.Request().Context(), projectId, clientId)
	if err != nil {
		return err
	}

	return c.JSON(200, instances)
}

func (self *LogsRouter) logsByClientHandler(c echo.Context) error {
	var params LogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		slog.Error("Bindings for logs by client not correct", "error", err)
		return c.JSON(400, "Bad request")
	}

	slog.Debug("Get logs by client", "params", params)

	var nextCursor *db.LastCursor
	if params.CursorTime != nil && params.CursorSequenceNumber != nil {
		nextCursor = &db.LastCursor{
			Timestamp:      time.UnixMilli(*params.CursorTime),
			SequenceNumber: *params.CursorSequenceNumber,
			IsBackward:     *params.Direction == "backward",
		}
	}

	logPage, err := self.logService.GetLogs(c.Request().Context(), db.LogPageRequest{
		ProjectId: params.ProjectId,
		ClientId:  params.ClientId,
		Instances: params.Instances,
		PageSize:  DEFAULT_PAGE_SIZE,
		LogLineId: params.LogLineId,
		Cursor:    nextCursor,
	})

	if err != nil {
		return err
	}

	props := pages.LogsPageProps{
		LogPage:   logPage,
		ClientId:  params.ClientId,
		ProjectId: params.ProjectId,
	}
	if params.Instances != nil && len(*params.Instances) > 0 {
		instances := (*params.Instances)
		props.InstanceId = &instances[0]
	}

	isHx := c.Request().Header.Get("HX-Request") == "true"
	if isHx && nextCursor != nil {
		return utils.Render(c, http.StatusOK, pages.LogPageLines(props))
	}

	return utils.Render(c, http.StatusOK, pages.LogsPage(props))
}

type SearchLogsByClientBinding struct {
	LogsByClientBinding
	Search string `query:"search"`
}

func (self *LogsRouter) searchLogs(c echo.Context) error {
	var params SearchLogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		slog.Error("Bindings for logs by client not correct", "error", err)
		return c.JSON(400, "Bad request")
	}

	var nextCursor db.LastCursor
	if params.CursorTime != nil && params.CursorSequenceNumber != nil {
		nextCursor = db.LastCursor{
			Timestamp:      time.UnixMilli(*params.CursorTime),
			SequenceNumber: *params.CursorSequenceNumber,
			IsBackward:     *params.Direction == "backward",
		}
	}

	slog.Debug("next", "cursor", nextCursor)
	logs, err := self.logService.SearchLogs(c.Request().Context(), params.Search, params.ProjectId, params.ClientId, params.Instances, DEFAULT_PAGE_SIZE, &nextCursor)

	if err != nil {
		return err
	}

	logsLen := len(logs)
	mappedLogs := make([]LogLine, logsLen)
	for i, log := range logs {
		mappedLogs[logsLen-i-1] = LogLine{
			log.ID.Hex(),
			log.Timestamp.UnixMilli(),
			log.LogLine,
			log.SequenceNumber,
			log.Client,
		}
	}

	return c.JSON(200, mappedLogs)
}

func NewLogsRouter(logService db.LogService, e *echo.Group) *LogsRouter {
	logsApi := e.Group("/logs/:projectId/:clientId")

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
