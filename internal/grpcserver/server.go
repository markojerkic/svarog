package grpcserver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/files"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

type GrpcServer struct {
	rpc.UnimplementedLoggAggregatorServer
	grpcServer *grpc.Server

	certificatesService serverauth.CertificateService
	env                 types.ServerEnv

	logIngestChannel chan db.LogLineWithIp
}

func (g *GrpcServer) BatchLog(ctx context.Context, batchLogs *rpc.Backlog) (*rpc.Void, error) {
	ipv4, err := getIp(ctx)
	if err != nil {
		return &rpc.Void{}, err
	}
	log.Debug("Received batch log", "size", int64(len(batchLogs.Logs)), "ip", ipv4)

	for _, logLine := range batchLogs.Logs {
		g.logIngestChannel <- db.LogLineWithIp{LogLine: logLine, Ip: ipv4}
	}
	return &rpc.Void{}, nil
}

// Log implements rpc.LoggAggregatorServer.
func (g *GrpcServer) Log(stream rpc.LoggAggregator_LogServer) error {
	for {
		logLine, err := stream.Recv()
		if err != nil {
			return err
		}
		ipv4, err := getIp(stream.Context())
		if err != nil {
			return err
		}

		g.logIngestChannel <- db.LogLineWithIp{LogLine: logLine, Ip: ipv4}
	}
}

func (i *GrpcServer) mustEmbedUnimplementedLoggAggregatorServer() {}

var _ rpc.LoggAggregatorServer = &GrpcServer{}

func getIp(ctx context.Context) (string, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return "", errors.New("failed to get peer from context")
	}

	// parse out ipv4 address from peer
	ipv6, _, err := net.SplitHostPort(peer.Addr.String())
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(ipv6)

	var ipv4 string
	if ip.IsLoopback() {
		ipv4 = "127.0.0.1"
	} else {
		ipv4 = ip.To4().String()
	}

	return ipv4, nil
}

func (gs *GrpcServer) getCaCert(ctx context.Context) (*x509.Certificate, error) {
	cert, _, err := gs.certificatesService.GetCaCertificate(ctx)
	if err != nil {
		if err.Error() == files.ErrFileNotFound {
			log.Warn("Ca cert file not found in db, creating a new one")
			gs.certificatesService.GenerateCaCertificate(ctx)
			return gs.getCaCert(ctx)
		}
		log.Error("Failed to get ca cert", "error", err)
		return nil, err
	}
	return cert, nil
}

func (gs *GrpcServer) Stop() {
	gs.grpcServer.Stop()
}

func (gs *GrpcServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", gs.env.GrpcServerPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	caCert, err := gs.getCaCert(context.Background())
	if err != nil {
		log.Fatal("Failed getting ca.crt", "err", err)
	}

	serverCertPath, cleanup, err := gs.certificatesService.GenerateCertificate(context.Background(), "svarog-server")
	if err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}
	defer cleanup()

	// Load server certificate and key
	serverCert, err := tls.LoadX509KeyPair(serverCertPath, serverCertPath)
	if err != nil {
		return fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Create certificate pool with CA certificate
	caPool := x509.NewCertPool()
	caPool.AddCert(caCert)

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}

	var opts []grpc.ServerOption = []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.UnaryInterceptor(NewAuthInterceptor().withInterceptor()),
	}
	gs.grpcServer = grpc.NewServer(opts...)

	rpc.RegisterLoggAggregatorServer(gs.grpcServer, gs)

	log.Info(fmt.Sprintf("Starting gRPC server on port %d, HTTP server on port %d", gs.env.GrpcServerPort, gs.env.HttpServerPort))

	return gs.grpcServer.Serve(lis)
}

func NewGrpcServer(certificatesService serverauth.CertificateService, env types.ServerEnv, logIngestChannel chan db.LogLineWithIp) *GrpcServer {
	return &GrpcServer{certificatesService: certificatesService, env: env, logIngestChannel: logIngestChannel}
}
