package gateway

import (
	"context"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	// "github.com/cometbft/cometbft/rpc/jsonrpc/types"
	"github.com/decentrio/gateway/config"
)

// JSON-RPC request format
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	ID      int         `json:"id"`
}

// JSON-RPC response format
type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result"`
	Error   interface{} `json:"error,omitempty"`
	ID      int         `json:"id"`
}

var (
	jsonRPCServers = make(map[uint16]*http.Server)
	activeJsonRPCRequestCount int32 
)

func Start_JSON_RPC_Server(server *Server) {
	fmt.Printf("Starting JSON-RPC server on port %d\n", server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/", trackRequestsMiddleware(handleJSONRPC))

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	mu.Lock()
	jsonRPCServers[server.Port] = srv
	mu.Unlock()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting JSON-RPC server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	Shutdown_JSON_RPC_Server(server)
}

func Shutdown_JSON_RPC_Server(server *Server) {
	mu.Lock()
	srv, exists := jsonRPCServers[server.Port]
	if !exists {
		mu.Unlock()
		return
	}
	delete(jsonRPCServers, server.Port)
	mu.Unlock()

	fmt.Printf("Waiting for %d active requests to complete before shutting down JSON-RPC server...\n", atomic.LoadInt32(&activeJsonRPCRequestCount))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down JSON-RPC server: %v\n", err)
	} else {
		fmt.Println("JSON-RPC server stopped.")
	}
}

func trackRequestsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&activeJsonRPCRequestCount, 1) 
		wg.Add(1)                       

		defer func() {
			wg.Done()                        
			atomic.AddInt32(&activeJsonRPCRequestCount, -1) 
		}()

		next(w, r)
	}
}

func handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var node *config.Node
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	var req JSONRPCRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received JSON-RPC request: Method=%s, Params=%v, ID=%d\n", req.Method, req.Params, req.ID)

	var height uint64
	if params, ok := req.Params.(map[string]interface{}); ok {
		if h, ok := params["height"].(float64); ok {
			height = uint64(h)
		}
	}


	if height > 0 {
		node = config.GetNodebyHeight(height)
		if node == nil {
			http.Error(w, "Node not found", http.StatusNotFound)
			return
		}
	}

	if node != nil {
		fmt.Printf("Forwarding to Node:", node.JSONRPC)

		reqForward, err := http.NewRequest("POST", node.JSONRPC, bytes.NewReader(body))
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		reqForward.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(reqForward)
		if err != nil {
			http.Error(w, "Failed to forward request", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
		return
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  fmt.Sprintf("Method %s executed successfully", req.Method),
		ID:      req.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
