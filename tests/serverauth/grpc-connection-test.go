package serverauth

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/grpcserver"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
)

type mockClient struct {
	connection *grpc.ClientConn
	stream     rpc.LoggAggregator_LogClient
	client     rpc.LoggAggregatorClient

	serverPort int
}

// BatchLog implements rpc.LoggAggregatorClient.
func (m *mockClient) BatchLog(ctx context.Context, in *rpc.Backlog) error {
	serverAddr := fmt.Sprintf("localhost:%d", m.serverPort)
	connection, err := grpc.NewClient(serverAddr)
	if err != nil {
		log.Fatal("Error connecting to server", "err", err)
	}

	client := rpc.NewLoggAggregatorClient(connection)

	_, err = client.BatchLog(ctx, in)
	return err
}

func (s *ServerauthSuite) TestGrpcConnection() {
	log.SetLevel(log.DebugLevel)

	randomFreePort, err := getFreePort()
	if err != nil {
		log.Fatal("Failed to get free tcp port", "err", err)
	}

	env := types.ServerEnv{
		GrpcServerPort: randomFreePort,
	}
	logIngestChan := make(chan db.LogLineWithIp)

	grpcServer := grpcserver.NewGrpcServer(s.certificatesService, env, logIngestChan)

	client := &mockClient{
		serverPort: randomFreePort,
	}

	go grpcServer.Start()
	defer grpcServer.Stop()

	line := &rpc.Backlog{}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line)
	assert.NoError(s.T(), err)
}

func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}
