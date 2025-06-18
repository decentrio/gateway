package gateway

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/register"
)

var (
	grpcServers            = make(map[uint16]*grpc.Server)
	activeGRPCRequestCount int32
)

func Start_GRPC_Server(server *Server) {
	fmt.Printf("Starting gRPC server on port %d\n", server.Port)
	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			fmt.Println("[ERROR] Metadata missing from request context")
			return nil, nil, status.Errorf(codes.Unimplemented, "Unknown method")
		}

		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		var selectedHost string

		heightStr := md.Get("x-cosmos-block-height")
		if len(heightStr) > 0 {
			height, err := strconv.ParseUint(heightStr[0], 10, 64)
			if err != nil {
				fmt.Println("[ERROR] Invalid x-cosmos-block-height:", heightStr[0])
				return nil, nil, status.Errorf(codes.InvalidArgument, "Invalid x-cosmos-block-height")
			}
			if node := config.GetNodebyHeight(height); node != nil {
				selectedHost = node.GRPC
			} else {
				fmt.Println("[ERROR] No matching backend found for height:", height)
				return nil, nil, status.Errorf(codes.InvalidArgument, "No matching backend found")
			}
		} else if node := config.GetNodebyHeight(0); node != nil {
			selectedHost = node.GRPC
		} else {
			fmt.Println("[ERROR] No available gRPC backends")
			return nil, nil, status.Errorf(codes.Unavailable, "No available gRPC backends")
		}

		fmt.Printf("Forwarding request %s to node: %s\n", fullMethodName, selectedHost)

		var conn *grpc.ClientConn
		var err error
		if strings.HasSuffix(selectedHost, ":443") {
			tlsConfig := &tls.Config{InsecureSkipVerify: true}
			conn, err = grpc.DialContext(ctx, selectedHost, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		} else {
			conn, err = grpc.DialContext(ctx, selectedHost, grpc.WithTransportCredentials(insecure.NewCredentials()))
		}

		if err != nil {
			fmt.Printf("[ERROR] Failed to connect to backend %s: %v\n", selectedHost, err)
		}
		return outCtx, conn, err
	}

	grpcServer := grpc.NewServer(
		grpc.UnknownServiceHandler(proxy.TransparentHandler(director)),
		grpc.ChainUnaryInterceptor(requestInterceptor),
		grpc.StreamInterceptor(requestStreamInterceptor),
	)

	// Register service
	register.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+strconv.Itoa(int(server.Port)))
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

var requestInterceptor grpc.UnaryServerInterceptor = func(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	atomic.AddInt32(&activeGRPCRequestCount, 1)
	wg.Add(1)
	defer func() {
		wg.Done()
		atomic.AddInt32(&activeGRPCRequestCount, -1)
	}()
	res, err := handler(ctx, req)
	return res, err
}

func requestStreamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	atomic.AddInt32(&activeGRPCRequestCount, 1)
	wg.Add(1)
	defer func() {
		wg.Done()
		atomic.AddInt32(&activeGRPCRequestCount, -1)
	}()
	err := handler(srv, ss)
	return err
}

func Shutdown_GRPC_Server(server *Server) {
	mu.Lock()
	grpcServer, ok := grpcServers[server.Port]
	mu.Unlock()

	if !ok {
		fmt.Println("[WARNING] No active gRPC server found to shut down.")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Waiting for %d active requests to complete before shutting down gRPC server...\n", atomic.LoadInt32(&activeGRPCRequestCount))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All active requests completed. Proceeding with shutdown...")
	case <-ctx.Done():
		fmt.Println("[WARNING] Timeout waiting for requests. Forcing shutdown...")
	}

	mu.Lock()
	grpcServer.GracefulStop()
	delete(grpcServers, server.Port)
	mu.Unlock()

	fmt.Println("gRPC server stopped.")
}
