package gateway

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/decentrio/gateway/config"
)

var (
	mu sync.Mutex
	wg sync.WaitGroup
)

type Server struct {
	Port uint16

	Start    func(server *Server)
	Shutdown func(server *Server)
}

type Gateway struct {
	RPC_Server         Server
	GRPC_Server        Server
	API_Server         Server
	JSON_RPC_Server    Server
	JSON_RPC_WS_Server Server
}

func NewGateway(cfg *config.Config) (*Gateway, error) {
	gw := &Gateway{}
	gw.RPC_Server = NewServer(cfg, "rpc")
	gw.GRPC_Server = NewServer(cfg, "grpc")
	gw.API_Server = NewServer(cfg, "api")
	gw.JSON_RPC_Server = NewServer(cfg, "jsonrpc")
	gw.JSON_RPC_WS_Server = NewServer(cfg, "jsonrpc_ws")
	return gw, nil
}

func NewServer(cfg *config.Config, serverType string) Server {
	new_server := &Server{}
	switch serverType {
	case "rpc":
		new_server.Port = cfg.Ports.RPC
		new_server.Start = Start_RPC_Server
		new_server.Shutdown = Shutdown_RPC_Server
	case "grpc":
		new_server.Port = cfg.Ports.GRPC
		new_server.Start = Start_GRPC_Server
		new_server.Shutdown = Shutdown_GRPC_Server
	case "api":
		new_server.Port = cfg.Ports.API
		new_server.Start = Start_API_Server
		new_server.Shutdown = Shutdown_API_Server
	case "jsonrpc":
		new_server.Port = cfg.Ports.JSONRPC
		new_server.Start = Start_JSON_RPC_Server
		new_server.Shutdown = Shutdown_JSON_RPC_Server
	case "jsonrpc_ws":
		new_server.Port = cfg.Ports.JSONRPC_WS
		new_server.Start = Start_JSON_RPC_WS_Server
		new_server.Shutdown = Shutdown_JSON_RPC_WS_Server
	default:
		fmt.Println("Invalid server type")
		os.Exit(1)
	}
	return *new_server
}

func (g *Gateway) Start() {
	go g.RPC_Server.Start(&g.RPC_Server)
	go g.GRPC_Server.Start(&g.GRPC_Server)
	go g.API_Server.Start(&g.API_Server)
	go g.JSON_RPC_Server.Start(&g.JSON_RPC_Server)
	go g.JSON_RPC_WS_Server.Start(&g.JSON_RPC_WS_Server)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop
	g.Shutdown()
}

func (g *Gateway) Shutdown() {
	var wg sync.WaitGroup
	servers := []*Server{
		&g.RPC_Server, &g.GRPC_Server, &g.API_Server, &g.JSON_RPC_Server, &g.JSON_RPC_WS_Server,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, server := range servers {
		wg.Add(1)
		go func(s *Server) {
			defer wg.Done()
			if err := shutdownWithTimeout(ctx, s); err != nil {
				fmt.Printf("Error shutting down %T: %v\n", s, err)
			}
		}(server)
	}

	wg.Wait()
	fmt.Println("All servers stopped.")
}

func shutdownWithTimeout(ctx context.Context, server *Server) error {
	done := make(chan error, 1)
	go func() {
		server.Shutdown(server)
		done <- nil
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("shutdown timeout")
	}
}
