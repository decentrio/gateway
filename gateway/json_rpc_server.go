package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

func handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var req JSONRPCRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON-RPC request", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received JSON-RPC request: Method=%s, Params=%v, ID=%d\n", req.Method, req.Params, req.ID)

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  fmt.Sprintf("Method %s executed successfully", req.Method),
		ID:      req.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func Shutdown_JSON_RPC_Server(server *Server) {
	fmt.Println("Shutting down server")
	os.Exit(0)
}