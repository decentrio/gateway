package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

func Start_JSON_RPC_Server(server *Server) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleJSONRPC)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting JSON-RPC server: %v", err)
		}
	}()

	fmt.Printf("JSON-RPC server is running on port %d\n", server.Port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Println("\nShutting down JSON-RPC server...")
	if err := srv.Close(); err != nil {
		log.Fatalf("Error shutting down JSON-RPC server: %v", err)
	}
	fmt.Println("JSON-RPC server stopped.")
}

func Shutdown_JSON_RPC_Server(server *Server) {
	fmt.Println("Shutting down JSON-RPC server")
	os.Exit(0)
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
