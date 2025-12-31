package serverauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/grpcserver"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type mockClient struct {
	connection *grpc.ClientConn
	stream     rpc.LoggAggregator_LogClient
	client     rpc.LoggAggregatorClient

	serverPort int
}

// BatchLog implements rpc.LoggAggregatorClient.
func (m *mockClient) BatchLog(ctx context.Context, in *rpc.Backlog, tlsConfig *tls.Config) error {
	serverAddr := fmt.Sprintf("0.0.0.0:%d", m.serverPort)

	connection, err := grpc.NewClient(serverAddr, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		panic(fmt.Errorf("Error connecting to server: %w", err))
	}

	client := rpc.NewLoggAggregatorClient(connection)

	_, err = client.BatchLog(ctx, in)
	return err
}

func (s *ServerauthSuite) TestGrpcConnection() {
	util.SetupLogger()

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		panic(fmt.Errorf("Failed to get free tcp port: %w", err))
	}

	env := types.ServerEnv{
		GrpcServerPort: randomFreePort,
	}
	logIngestChan := make(chan db.LogLineWithHost)

	grpcServer := grpcserver.NewGrpcServer(s.certificatesService, s.projectsService, env, logIngestChan)

	client := &mockClient{
		serverPort: randomFreePort,
	}

	go grpcServer.Start()
	defer grpcServer.Stop()

	line := &rpc.Backlog{}

	clientCertPath, cleanup, err := s.certificatesService.GenerateCertificate(context.Background(), "mock-client")
	if err != nil {
		panic(fmt.Errorf("failed to generate server certificate: %w", err))
	}
	defer cleanup()

	// Load client certificate and key from the PEM file
	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientCertPath)
	if err != nil {
		panic(fmt.Errorf("failed to load client certificate: %w", err))
	}

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	if err != nil {
		panic(fmt.Errorf("Failed to get ca.crt: %w", err))
	}

	// Create certificate pool and add CA certificate
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
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
