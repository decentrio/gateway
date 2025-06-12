package gateway

import (
	"context"
	"crypto/tls"
	// "encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/decentrio/gateway/config"
	// "github.com/decentrio/gateway/gateway"
	generic "github.com/decentrio/gateway/proto"
	"github.com/mwitkow/grpc-proxy/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
    grpcServers            = make(map[uint16]*grpc.Server)
    activeGRPCRequestCount int32
)

// Context key for block height
type ctxKeyHeight struct{}

// --- Interceptor: Extract height from any body type ---
var requestInterceptor grpc.UnaryServerInterceptor = func(
    ctx context.Context,
    req interface{},
    info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler,
) (interface{}, error) {
    var height uint64 = 0

    // Try to extract "height" from any body type
    if genericReq, ok := req.(*generic.GenericRequest); ok {
        body := genericReq.Body.AsMap()
        if h, exists := body["height"]; exists {
            switch v := h.(type) {
            case float64:
                height = uint64(v)
            case string:
                if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
                    height = parsed
                }
            }
        }
    }

    ctx = context.WithValue(ctx, ctxKeyHeight{}, height)

    atomic.AddInt32(&activeGRPCRequestCount, 1)
    wg.Add(1)
    defer func() {
        wg.Done()
        atomic.AddInt32(&activeGRPCRequestCount, -1)
    }()
    return handler(ctx, req)
}

// --- Stream Interceptor: Extract height from first message and store in context ---
type wrappedServerStream struct {
    grpc.ServerStream
    ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
    if w.ctx != nil {
        return w.ctx
    }
    return w.ServerStream.Context()
}

func (w *wrappedServerStream) RecvMsg(m interface{}) error {
    err := w.ServerStream.RecvMsg(m)
    if err == nil {
        fmt.Printf("RecvMsg called")
        var height uint64 = 0
        if req, ok := m.(*generic.GenericRequest); ok {
            fmt.Printf("Extracting height from GenericRequest\n")
            body := req.Body.AsMap()
            if h, exists := body["height"]; exists {
                switch v := h.(type) {
                case float64:
                    height = uint64(v)
                case string:
                    if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
                        height = parsed
                    }
                }

                fmt.Printf("Found Height: %d\n", height)
            }
        } else {
            fmt.Printf("No GenericRequest found, using default height 0\n")
        }

        w.ctx = context.WithValue(w.ServerStream.Context(), ctxKeyHeight{}, height)
    }
    return err
}

func requestStreamInterceptor(
    srv interface{},
    ss grpc.ServerStream,
    info *grpc.StreamServerInfo,
    handler grpc.StreamHandler,
) error {
    fmt.Println("Stream request received:", info.FullMethod)
    atomic.AddInt32(&activeGRPCRequestCount, 1)
    wg.Add(1)
    defer func() {
        wg.Done()
        atomic.AddInt32(&activeGRPCRequestCount, -1)
    }()
    wrapped := &wrappedServerStream{ServerStream: ss}
    return handler(srv, wrapped)
}

// --- Director: Use height from context ---
func Start_GRPC_Server(server *Server) {
    fmt.Printf("Starting gRPC server on port %d\n", server.Port)
    director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            fmt.Println("[ERROR] Metadata missing from request context")
            return nil, nil, status.Errorf(codes.Unimplemented, "Unknown method")
        }

        outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
        var height uint64 = 0
        if h, ok := ctx.Value(ctxKeyHeight{}).(uint64); ok {
            height = h
        }

        var selectedHost string
        if node := config.GetNodebyHeight(height); node != nil {
            selectedHost = node.GRPC
        } else {
            fmt.Println("[ERROR] No matching backend found for height:", height)
            return nil, nil, status.Errorf(codes.InvalidArgument, "No matching backend found")
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
        grpc.UnaryInterceptor(requestInterceptor),
        grpc.StreamInterceptor(requestStreamInterceptor),
    )

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