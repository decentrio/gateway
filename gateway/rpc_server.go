package gateway

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/decentrio/gateway/config"
	"github.com/decentrio/gateway/httpUtils"
)

var (
	rpcServers = make(map[uint16]*http.Server)
)

func Start_RPC_Server(server *Server) {
	fmt.Printf("Starting RPC server on port %d\n", server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleRequest)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	mu.Lock()
	rpcServers[server.Port] = srv
	mu.Unlock()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-stop
		Shutdown_RPC_Server(server)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Printf("Failed to start RPC server: %v\n", err)
	}
}

func Shutdown_RPC_Server(server *Server) {
	mu.Lock()
	srv, exists := rpcServers[server.Port]
	if !exists {
		mu.Unlock()
		return
	}
	delete(rpcServers, server.Port)
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down RPC server: %v\n", err)
	} else {
		fmt.Println("RPC server stopped.")
	}
}

func (server *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received RPC query: %s\n", r.URL.Path)
	
	var node *config.Node

	switch r.URL.Path {
	case "/abci_info",
		 "/broadcast_evidence",
		 "/broadcast_tx_async",
		 "/broadcast_tx_commit",
		 "/broadcast_tx_sync",
		 "/consensus_state",
		 "/dump_consensus_state",
		 "/genesis",
		 "/genesis_chunked",
		 "/health",
		 "/net_info",
		 "/num_unconfirmed_txs",
		 "/status",
		 "/subscribe",
		 "/unsubscribe",
		 "/unsubscribe_all",
		 "/websocket":
		node = config.GetNodebyHeight(0)
		if node == nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
		httpUtils.FowardRequest(w, r, node.RPC)
		return
	case "/abci_query",
		 "/block",
		 "/block_results",
		 "/commit",
		 "/consensus_params",
		 "/header",
		 "/validators":
		height := r.URL.Query().Get("height")
		if height != "" {
			h, err := strconv.ParseUint(height, 10, 64)
			if err != nil {
				http.Error(w, "Invalid height", http.StatusBadRequest)
				return
			}
	
			node = config.GetNodebyHeight(h)
			if node == nil {
				http.Error(w, "Node not found", http.StatusNotFound)
				return
			} else {
				fmt.Println("Node: ", node.RPC)
			}
		} else {
			node = config.GetNodebyHeight(0)
			if node == nil {
				http.Error(w, "Node not found", http.StatusNotFound)
				return
			} else {
				fmt.Println("Node: ", node.RPC)
			}
		}
	
		httpUtils.FowardRequest(w, r, node.RPC)
		return
	case "blockchain":
		height := r.URL.Query().Get("maxHeight")
		if height != "" {
			h, err := strconv.ParseUint(height, 10, 64)
			if err != nil {
				http.Error(w, "Invalid height", http.StatusBadRequest)
				return
			}
	
			node = config.GetNodebyHeight(h)
			if node == nil {
				http.Error(w, "Node not found", http.StatusNotFound)
				return
			} else {
				fmt.Println("Node: ", node.RPC)
			}
		} else {
			node = config.GetNodebyHeight(0)
			if node == nil {
				http.Error(w, "Node not found", http.StatusNotFound)
				return
			} else {
				fmt.Println("Node: ", node.RPC)
			}
		}
	
		httpUtils.FowardRequest(w, r, node.RPC)
		return
	case "/block_by_hash",
		 "/block_search",
		 "/check_tx",
		 "/header_by_hash",
		 "/tx",
		 "/tx_search":
		var success bool
		RPC_nodes := config.GetNodesByType("rpc")
		for _, node := range RPC_nodes {
			if httpUtils.CheckRequest(w, r, node) {
				success = true
				break
			}
		}
		if !success {
			http.Error(w, "Invalid request", http.StatusInternalServerError)
		}
		return
	default:
		http.Error(w, "Invalid path", http.StatusNotFound)
		return
	}
}