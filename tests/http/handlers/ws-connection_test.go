package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/markojerkic/svarog/internal/lib/auth"
	"github.com/markojerkic/svarog/internal/server/http/handlers"
	"github.com/markojerkic/svarog/internal/server/live"
	"github.com/markojerkic/svarog/internal/server/logstore"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestLiveWebSocketResyncAndHandshake(t *testing.T) {
	broker, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: server.RANDOM_PORT, NoSigs: true})
	require.NoError(t, err)
	broker.Start()
	t.Cleanup(broker.Shutdown)
	nc, err := nats.Connect(broker.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	gateway := live.New(nc)
	t.Cleanup(gateway.Close)

	e := echo.New()
	handlers.NewWsConnectionRouter(gateway, &projectAccessAuthStub{allowed: true}, &projectAccessProjectsStub{belongs: true},
		loggedInGroup(e, &auth.LoggedInUser{ID: "user", Role: auth.USER}))
	httpServer := httptest.NewServer(e)
	t.Cleanup(httpServer.Close)
	url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws/" + projectID + "/" + clientID
	ws, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	// An invalidation immediately after the handshake must reach the active subscription.
	gateway.AfterUncertainCommit([]logstore.Scope{{ProjectID: uuid.MustParse(projectID), ClientID: uuid.MustParse(clientID)}})
	require.NoError(t, ws.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, _, err = ws.ReadMessage()
	require.True(t, websocket.IsCloseError(err, 4001), "expected refresh-required close, got %v", err)

	gateway.Close()
	_, response, err := websocket.DefaultDialer.Dial(url, nil)
	require.ErrorIs(t, err, websocket.ErrBadHandshake, "an unavailable subscription must not look like a successful reconnect")
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	response.Body.Close()
}
