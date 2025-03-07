// package gateway

// import (
// 	"fmt"
// 	// "net/http"
// 	"os"

// 	// "google.golang.org/grpc"
// 	// "github.com/decentrio/gateway/config"
// )

// func Start_GRPC_Server(server *Server) {
// 	fmt.Printf("Starting server on port %d\n", server.Port)
// 	// http.HandleFunc("/", server.handleRequest)
// 	// http.HandleFunc("/",
// 	// 	func(w http.ResponseWriter, r *http.Request) {
// 	// 		fmt.Fprintf(w, "Hello, you've requested: %s\n", r.URL.Path)
// 	// 	})
// 	// err := http.ListenAndServe(fmt.Sprintf(":%d", server.Port), nil)
// 	// if err != nil {
// 	// 	fmt.Printf("Error starting server: %v\n", err)
// 	// }
// }

// func Shutdown_GRPC_Server(server *Server) {
// 	fmt.Println("Shutting down server")
// 	os.Exit(0)
// }

package gateway

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"

	"github.com/decentrio/gateway/config"
	pb "github.com/decentrio/gateway/proto"
	"google.golang.org/grpc"
)

// GRPCServer struct

type GRPCServer struct {
	pb.UnimplementedGatewayServiceServer
}

// Start_GRPC_Server starts the gRPC server
func Start_GRPC_Server(server *Server) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", server.Port))
	if err != nil {
		fmt.Printf("Failed to listen: %v\n", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterGatewayServiceServer(grpcServer, &GRPCServer{})

	fmt.Printf("Starting gRPC server on port %d\n", server.Port)
	if err := grpcServer.Serve(lis); err != nil {
		fmt.Printf("Failed to serve: %v\n", err)
	}
}

// Shutdown_GRPC_Server shuts down the gRPC server
func Shutdown_GRPC_Server(server *Server) {
	fmt.Println("Shutting down gRPC server")
	os.Exit(0)
}

// GetNodeInfo handles the gRPC request
func (s *GRPCServer) GetNodeInfo(ctx context.Context, req *pb.NodeRequest) (*pb.NodeResponse, error) {
	var node *config.Node

	if req.Height != "" {
		h, err := strconv.ParseUint(req.Height, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid height: %v", err)
		}
		node = config.GetNodebyHeight(h)
		if node == nil {
			return nil, fmt.Errorf("node not found")
		}
		fmt.Println("Node: ", node.RPC)
	}

	return &pb.NodeResponse{RpcUrl: node.RPC}, nil
}
