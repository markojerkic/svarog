package handlers

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
)

type LogsByClientBinding struct {
	ClientId   string  `param:"clientId"`
	CursorTime *int64  `query:"cursorTime"`
	CursorId   *string `query:"cursorId"`
	Direction  *string `query:"direction"`
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

func (self *LogsRouter) logsByClientHandler(c echo.Context) error {
	var params LogsByClientBinding

	err := c.Bind(&params)
	if err != nil {
		slog.Error("Bindings for logs by client not correct", err)
		return c.String(400, "<h1>400 Bad Request</h1>")
	}

	slog.Debug("params", slog.Any("params", params))

	var nextCursor db.LastCursor
	if params.CursorTime != nil && params.CursorId != nil {
		nextCursor = db.LastCursor{
			Timestamp: time.UnixMilli(*params.CursorTime),
			ID:        *params.CursorId,
            IsBackward: *params.Direction == "backward",
		}
	}

	slog.Debug("next", slog.Any("cursor", nextCursor))
	logs, err := self.logRepository.GetLogs(params.ClientId, &nextCursor)

	if err != nil {
		return c.String(400, "<h1>400 Bad Request</h1>")
	}

	mappedLogs := make([]LogLine, len(logs))
	for i, log := range logs {
		mappedLogs[i] = LogLine{
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
