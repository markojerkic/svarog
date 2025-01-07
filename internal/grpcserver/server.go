package grpcserver

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/charmbracelet/log"
	"github.com/markojerkic/svarog/internal/lib/serverauth"
	rpc "github.com/markojerkic/svarog/internal/proto"
	"github.com/markojerkic/svarog/internal/server/db"
	"github.com/markojerkic/svarog/internal/server/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

type GrpcServer struct {
	rpc.UnimplementedLoggAggregatorServer

	certificatesService serverauth.CertificateService
	env                 types.ServerEnv

	logIngestChannel chan db.LogLineWithIp
}

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

func (gs *GrpcServer) Start() error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", gs.env.GrpcServerPort))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	rpc.RegisterLoggAggregatorServer(grpcServer, gs)

	log.Info(fmt.Sprintf("Starting gRPC server on port %d, HTTP server on port %d", gs.env.GrpcServerPort, gs.env.HttpServerPort))

	grpcServer.Serve(lis)

	return nil
}

func NewGrpcServer(certificatesService serverauth.CertificateService, env types.ServerEnv, logIngestChannel chan db.LogLineWithIp) *GrpcServer {
	return &GrpcServer{certificatesService: certificatesService, env: env, logIngestChannel: logIngestChannel}
}
