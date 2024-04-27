package handlers

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
	"github.com/markojerkic/svarog/server/views"
)

type LogsByClientBinding struct {
	ClientId string `param:"clientId"`
	Cursor   *int64 `query:"cursor"`
}

func LogsByClient(logRepository db.LogRepository) echo.HandlerFunc {
	return func(c echo.Context) error {
		slog.Debug("LogsByClient request", slog.String("clientId", c.Param("clientId")), slog.Any("query", c.QueryParams()))
		var params LogsByClientBinding

		err := c.Bind(&params)
		if err != nil {
			slog.Error("Bindings for logs by client not correct", err)
			return c.String(400, "<h1>400 Bad Request</h1>")
		}

		slog.Debug("LogsByClientBinding", slog.Any("params", params))

		var nextCursor time.Time
		if params.Cursor != nil {
			nextCursor = time.Unix(*params.Cursor, 0)
		}

		logs, err := logRepository.GetLogs(params.ClientId, &nextCursor)

		if err != nil {
			return c.String(400, "<h1>400 Bad Request</h1>")
		}

		slog.Debug("LogsByClient", slog.Any("logs", logs))

		return views.LogsByClientId(params.ClientId, logs).Render(c.Request().Context(), c.Response().Writer)
	}
}
