package handlers

import (
	"log/slog"
	"time"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
	"github.com/markojerkic/svarog/db"
	"github.com/markojerkic/svarog/cmd/server/views"
)

type LogsByClientBinding struct {
	ClientId   string  `param:"clientId"`
	CursorTime *int64  `query:"cursorTime"`
	CursorId   *string `query:"cursorId"`
}

func selectView(c echo.Context) func(clientId string, cursor *int64, logs []db.StoredLog) templ.Component {
	if c.Request().Header.Get("Hx-Request") == "true" {
		return views.LogBlock
	}
	return views.LogsByClientId
}

func LogsByClient(logRepository db.LogRepository) echo.HandlerFunc {
	return func(c echo.Context) error {
		var params LogsByClientBinding

		err := c.Bind(&params)
		if err != nil {
			slog.Error("Bindings for logs by client not correct", err)
			return c.String(400, "<h1>400 Bad Request</h1>")
		}

		var nextCursor db.LastCursor
		if params.CursorTime != nil && params.CursorId != nil {
			nextCursor = db.LastCursor{
				Timestamp: time.UnixMilli(*params.CursorTime),
				ID:        *params.CursorId,
			}
		}

		slog.Debug("next", slog.Any("cursor", nextCursor))
		logs, err := logRepository.GetLogs(params.ClientId, &nextCursor)

		if err != nil {
			return c.String(400, "<h1>400 Bad Request</h1>")
		}

		view := selectView(c)
		return view(params.ClientId, params.CursorTime, logs).Render(c.Request().Context(), c.Response().Writer)
	}
}
