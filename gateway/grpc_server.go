package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/decentrio/gateway/config"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	grpcServers = make(map[uint16]*grpc.Server)
)

func Start_GRPC_Server(server *Server) {
	fmt.Printf("Starting gRPC server on port %d\n", server.Port)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
	}

	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		if !ok {
			return nil, nil, status.Errorf(codes.Unimplemented, "Unknown method")
		}

		var selectedHost string
		m0 := md["x-cosmos-block-height"]
		if len(m0) > 0 {
			height, err := strconv.ParseUint(m0[0], 10, 64)
			if err != nil {
				return nil, nil, status.Errorf(codes.InvalidArgument, "Invalid x-cosmos-block-height")
			}

			node := config.GetNodebyHeight(height)
			if node == nil {
				return nil, nil, status.Errorf(codes.InvalidArgument, "No matching backend found")
			}
			selectedHost = node.GRPC
		} else {
			node := config.GetNodebyHeight(0)
			if node != nil {
				selectedHost = node.GRPC
			} else {
				return nil, nil, status.Errorf(codes.Unavailable, "No available gRPC backends")
			}
		}

		fmt.Printf("Forwarding request %s to node: %s\n", fullMethodName, selectedHost)

		if strings.HasSuffix(selectedHost, ":443") {
			conn, err := grpc.DialContext(ctx, selectedHost, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
			return outCtx, conn, err
		}

		conn, err := grpc.DialContext(ctx, selectedHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return outCtx, conn, err
	}

	grpcServer := grpc.NewServer(grpc.UnknownServiceHandler(proxy.TransparentHandler(director)))
	grpcPort := server.Port
	if grpcPort == 0 {
		grpcPort = 9090
	}
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(int(grpcPort)))
	if err != nil {
		panic(err)
	}

	mu.Lock()
	grpcServers[server.Port] = grpcServer
	mu.Unlock()

	go func() {
		_ = grpcServer.Serve(lis)
	}()
}

func Shutdown_GRPC_Server(server *Server) {
	mu.Lock()
	defer mu.Unlock()

	if grpcServer, ok := grpcServers[server.Port]; ok {
		grpcServer.GracefulStop()
		delete(grpcServers, server.Port)
		fmt.Println("gRPC server stopped.")
	} else {
		fmt.Println("No active gRPC server found to shut down.")
	}
}
