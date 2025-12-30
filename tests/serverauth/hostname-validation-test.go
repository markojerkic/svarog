package serverauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"time"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/grpcserver"
	"github.com/markojerkic/svarog/internal/rpc"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"github.com/stretchr/testify/assert"
)

func (s *ServerauthSuite) TestGrpcConnection_WrongHostname() {
	log.SetLevel(log.DebugLevel)

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		log.Fatal("Failed to get free tcp port", "err", err)
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
		log.Fatal("failed to generate server certificate", "err", err)
	}
	defer cleanup()

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientCertPath)
	if err != nil {
		log.Fatal("failed to load client certificate", "err", err)
	}

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	if err != nil {
		log.Fatal("Failed to get ca.crt", "err", err)
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   "wrong.hostname.example",
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "certificate is valid for")
}

func (s *ServerauthSuite) TestGrpcConnection_CorrectHostname() {
	log.SetLevel(log.DebugLevel)

	err := s.certificatesService.GenerateCaCertificate(context.Background())
	assert.NoError(s.T(), err)

	randomFreePort, err := getFreePort()
	if err != nil {
		log.Fatal("Failed to get free tcp port", "err", err)
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
		log.Fatal("failed to generate server certificate", "err", err)
	}
	defer cleanup()

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientCertPath)
	if err != nil {
		log.Fatal("failed to load client certificate", "err", err)
	}

	caCert, _, err := s.certificatesService.GetCaCertificate(context.Background())
	if err != nil {
		log.Fatal("Failed to get ca.crt", "err", err)
	}

	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caPool,
		ServerName:   "0.0.0.0",
	}

	timeoutContext, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = client.BatchLog(timeoutContext, line, tlsConfig)
	assert.NoError(s.T(), err)
}
