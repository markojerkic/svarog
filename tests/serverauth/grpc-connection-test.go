package serverauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	clientCertPath, cleanup, err := s.certificatesService.GenerateCertificate(context.Background(), "mock-client")
	if err != nil {
		log.Fatal("failed to generate server certificate", "err", err)
	}
	defer cleanup()

	// Load client certificate and key from the PEM file
	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientCertPath)
	if err != nil {
		log.Fatal("failed to load client certificate", "err", err)
	}

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	if err != nil {
		log.Fatal("Failed to get ca.crt", "err", err)
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
