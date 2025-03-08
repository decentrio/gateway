package gateway

import (
	"fmt"
	"os"

	"github.com/decentrio/gateway/config"
)

type Server struct {
	Port uint16
	
	Start func(server *Server)
	Shutdown func(server *Server)
}

type Gateway struct {
	RPC_Server Server 
	GRPC_Server Server
	API_Server Server
	JSON_RPC_Server Server
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

	select {}
}


func (g *Gateway) Shutdown() {
	g.RPC_Server.Shutdown(&g.RPC_Server)
	g.GRPC_Server.Shutdown(&g.GRPC_Server)
	g.API_Server.Shutdown(&g.API_Server)
	g.JSON_RPC_Server.Shutdown(&g.JSON_RPC_Server)
	g.JSON_RPC_WS_Server.Shutdown(&g.JSON_RPC_WS_Server)
}
