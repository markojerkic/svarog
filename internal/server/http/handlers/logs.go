package handlers

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
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
	ClientId             string    `param:"clientId"`
	CursorTime           *int64    `query:"cursorTime"`
	CursorSequenceNumber *int      `query:"cursorSequenceNumber"`
	Direction            *string   `query:"direction"`
	Instances            *[]string `query:"instance"`
	LogLineId            *string   `query:"logLine"`
}

func (self *LogsRouter) instancesByClientHandler(c echo.Context) error {
	clientId := c.Param("clientId")
	if clientId == "" {
		return c.JSON(400, "No client id")
	}
	log.Debug("Getting instances by client", "clientId", clientId)

	instances, err := self.logService.GetInstances(c.Request().Context(), clientId)
	if err != nil {
		return err
	}

	return c.JSON(200, instances)
}

func (self *LogsRouter) logsByClientHandler(c echo.Context) error {
	var params LogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		log.Error("Bindings for logs by client not correct", "error", err)
		return c.JSON(400, "Bad request")
	}

	log.Debug("Get logs by client", "params", params)

	var nextCursor db.LastCursor
	if params.CursorTime != nil && params.CursorSequenceNumber != nil {
		nextCursor = db.LastCursor{
			Timestamp:      time.UnixMilli(*params.CursorTime),
			SequenceNumber: *params.CursorSequenceNumber,
			IsBackward:     *params.Direction == "backward",
		}
	}

	logs, err := self.logService.GetLogs(c.Request().Context(), params.ClientId, params.Instances, DEFAULT_PAGE_SIZE, params.LogLineId, &nextCursor)

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

type SearchLogsByClientBinding struct {
	LogsByClientBinding
	Search string `query:"search"`
}

func (self *LogsRouter) clientsHandler(c echo.Context) error {
	clients, err := self.logService.GetClients(c.Request().Context())
	if err != nil {
		return err
	}

	return c.JSON(200, clients)
}

func (self *LogsRouter) searchLogs(c echo.Context) error {
	var params SearchLogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		log.Error("Bindings for logs by client not correct", "error", err)
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

	log.Debug("next", "cursor", nextCursor)
	logs, err := self.logService.SearchLogs(c.Request().Context(), params.Search, params.ClientId, params.Instances, DEFAULT_PAGE_SIZE, &nextCursor)

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
	logsApi := e.Group("/logs")

	logsRouter := &LogsRouter{
		logService:   logService,
		parentRouter: e,
		api:          logsApi,
	}

	logsRouter.api.GET("/clients", logsRouter.clientsHandler)
	logsRouter.api.GET("/:clientId", logsRouter.logsByClientHandler)
	logsRouter.api.GET("/:clientId/instances", logsRouter.instancesByClientHandler)
	logsRouter.api.GET("/:clientId/search", logsRouter.searchLogs)

	return logsRouter
}
