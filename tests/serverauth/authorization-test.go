package serverauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	"log/slog"

	"github.com/markojerkic/svarog/internal/lib/util"
	"github.com/markojerkic/svarog/internal/grpcserver"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *ServerauthSuite) TestBatchLog_UnauthorizedClient() {
	util.SetupLogger()

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		panic(fmt.Errorf("Failed to get free tcp port: %w", err))
	}

	project, err := s.projectsService.CreateProject(context.Background(), "test-project", []string{"authorized-client"})
	assert.NoError(s.T(), err)

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

	clientCertPath, cleanup, err := s.certificatesService.GenerateCertificate(context.Background(), "unauthorized-client")
	if err != nil {
		panic(fmt.Errorf("failed to generate client certificate: %w", err))
	}
	defer cleanup()

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientCertPath)
	if err != nil {
		panic(fmt.Errorf("failed to load client certificate: %w", err))
	}

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	if err != nil {
		panic(fmt.Errorf("Failed to get ca.crt: %w", err))
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   "0.0.0.0",
	}

	line := &rpc.Backlog{
		Logs: []*rpc.LogLine{
			{
				Client:    "some-client",
				Message:   "test message",
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
	assert.Error(s.T(), err)

	st, ok := status.FromError(err)
	assert.True(s.T(), ok)
	assert.Equal(s.T(), codes.PermissionDenied, st.Code())

	s.projectsService.DeleteProject(context.Background(), project.ID.Hex())
}

func (s *ServerauthSuite) TestBatchLog_AuthorizedClient() {
	util.SetupLogger()

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		panic(fmt.Errorf("Failed to get free tcp port: %w", err))
	}

	project, err := s.projectsService.CreateProject(context.Background(), "test-project-2", []string{"authorized-client"})
	assert.NoError(s.T(), err)

	env := types.ServerEnv{
		GrpcServerPort: randomFreePort,
	}
	logIngestChan := make(chan db.LogLineWithHost, 10)

	grpcServer := grpcserver.NewGrpcServer(s.certificatesService, s.projectsService, env, logIngestChan)

	client := &mockClient{
		serverPort: randomFreePort,
	}

	go grpcServer.Start()
	defer grpcServer.Stop()

	clientCertPath, cleanup, err := s.certificatesService.GenerateCertificate(context.Background(), project.ID.Hex())
	if err != nil {
		panic(fmt.Errorf("failed to generate client certificate: %w", err))
	}
	defer cleanup()

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientCertPath)
	if err != nil {
		panic(fmt.Errorf("failed to load client certificate: %w", err))
	}

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	if err != nil {
		panic(fmt.Errorf("Failed to get ca.crt: %w", err))
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   "0.0.0.0",
	}

	line := &rpc.Backlog{
		Logs: []*rpc.LogLine{
			{
				Client:    "authorized-client",
				Message:   "test message",
				Timestamp: timestamppb.New(time.Now()),
			},
		},
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
	assert.NoError(s.T(), err)

	s.projectsService.DeleteProject(context.Background(), project.ID.Hex())
}
