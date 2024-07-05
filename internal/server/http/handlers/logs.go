package handlers

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/internal/server/db"
)

type LogsByClientBinding struct {
	ClientId             string  `param:"clientId"`
	CursorTime           *int64  `query:"cursorTime"`
	CursorSequenceNumber *int    `query:"sequenceNumber"`
	Direction            *string `query:"direction"`
}

type LogLine struct {
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Content   string `json:"content"`
}

type LogsRouter struct {
	logRepository db.LogRepository
	parentRouter  *echo.Group
	api           *echo.Group
}

var DEFAULT_PAGE_SIZE = int64(300)

func (self *LogsRouter) logsByClientHandler(c echo.Context) error {
	var params LogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		slog.Error("Bindings for logs by client not correct", slog.Any("error", err))
		return c.JSON(400, "Bad request")
	}

	slog.Debug("params", slog.Any("params", params))

	var nextCursor db.LastCursor
	if params.CursorTime != nil && params.CursorSequenceNumber != nil {
		nextCursor = db.LastCursor{
			Timestamp:      time.UnixMilli(*params.CursorTime),
			SequenceNumber: *params.CursorSequenceNumber,
			IsBackward:     *params.Direction == "backward",
		}
	}

	slog.Debug("next", slog.Any("cursor", nextCursor))
	logs, err := self.logRepository.GetLogs(params.ClientId, DEFAULT_PAGE_SIZE, &nextCursor)

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
		}
	}

	return c.JSON(200, mappedLogs)
}

func NewLogsRouter(logRepository db.LogRepository, e *echo.Group) *LogsRouter {
	logsApi := e.Group("/logs")

	logsRouter := &LogsRouter{
		logRepository: logRepository,
		parentRouter:  e,
		api:           logsApi,
	}

	logsRouter.api.GET("/:clientId", logsRouter.logsByClientHandler)

	return logsRouter
}
