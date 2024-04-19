package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	rpc "github.com/markojerkic/svarog/proto"
	"google.golang.org/grpc/credentials"
)

type ImplementedServer struct{}

// GetFeature implements rpc.RouteGuideServer.
func (i *ImplementedServer) GetFeature(context.Context, *rpc.Point) (*rpc.Feature, error) {
	return &rpc.Feature{}, nil
}

// ListFeatures implements rpc.RouteGuideServer.
func (i *ImplementedServer) ListFeatures(rect *rpc.Rectangle, stream rpc.RouteGuide_ListFeaturesServer) error {
	stream.Send(&rpc.Feature{})

	return nil
}

// RecordRoute implements rpc.RouteGuideServer.
func (i *ImplementedServer) RecordRoute(rpc.RouteGuide_RecordRouteServer) error {
	panic("unimplemented")
}

// RouteChat implements rpc.RouteGuideServer.
func (i *ImplementedServer) RouteChat(rpc.RouteGuide_RouteChatServer) error {
	panic("unimplemented")
}

// mustEmbedUnimplementedRouteGuideServer implements rpc.RouteGuideServer.
func (i *ImplementedServer) mustEmbedUnimplementedRouteGuideServer() {
	panic("unimplemented")
}

var server rpc.RouteGuideServer

func main() {
	println("Hello, World!")

	scanner := bufio.NewScanner(os.Stdin)
	// rpc.RegisterRouteGuideServer(grpc.ServiceRegistrar, )

    credentials.NewClientTLSFromCert(nil, "")

	nurLines := 0

	for scanner.Scan() {
		line := scanner.Text()

		fmt.Printf("[%d] Line entered: '%s'\n", nurLines, line)
		nurLines++

	}

}
