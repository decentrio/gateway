package gateway

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/decentrio/gateway/config"
	pb "github.com/decentrio/gateway/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	pb.UnimplementedGatewayServiceServer
}

func Start_GRPC_Server(server *Server) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", server.Port))
	if err != nil {
		log.Fatalf("Failed to listen on port %d: %v", server.Port, err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()
	reflection.Register(grpcServer)
	pb.RegisterGatewayServiceServer(grpcServer, &GRPCServer{})

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	fmt.Printf("gRPC server is running on port %d\n", server.Port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Println("\nShutting down gRPC server...")
	grpcServer.GracefulStop()
	fmt.Println("gRPC server stopped.")
}

func Shutdown_GRPC_Server(server *Server) {
	fmt.Println("Shutting down gRPC server")
	os.Exit(0)
}

func (s *GRPCServer) HandleRequest(ctx context.Context, req *pb.GatewayRequest) (*pb.GatewayResponse, error) {
	fmt.Println("Received gRPC request for height:", req.Height)

	var node *config.Node
	heightStr := req.Height

	if heightStr != "" {
		h, err := strconv.ParseUint(heightStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid height: %v", err)
		}

		node = config.GetNodebyHeight(h)
		if node == nil {
			return nil, fmt.Errorf("node not found")
		} else {
			fmt.Println("Node gRPC URL:", node.GRPC)
		}
	}

	response := &pb.GatewayResponse{
		Message: fmt.Sprintf("Request forwarded to node at %s", node.GRPC),
	}

	return response, nil
}
