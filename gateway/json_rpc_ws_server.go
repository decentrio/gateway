package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

var (
	jsonRPCWSServers = make(map[uint16]*http.Server)
	upgrader         = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			log.Printf("WebSocket request from: %s", r.Host)
			return true
		},
	}
	activeJsonRPCWSRequestCount int32
)

func Start_JSON_RPC_WS_Server(server *Server) {
	fmt.Printf("Starting JSON-RPC WebSocket server on port %d\n", server.Port)

	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", handleWebSocket)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	mu.Lock()
	jsonRPCWSServers[server.Port] = srv
	mu.Unlock()

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting JSON-RPC WebSocket server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	Shutdown_JSON_RPC_WS_Server(server)
}

func Shutdown_JSON_RPC_WS_Server(server *Server) {
	mu.Lock()
	srv, exists := jsonRPCWSServers[server.Port]
	if !exists {
		mu.Unlock()
		return
	}
	delete(jsonRPCWSServers, server.Port)
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Printf("Waiting for %d active WebSocket requests to complete before shutting down...\n", atomic.LoadInt32(&activeJsonRPCWSRequestCount))

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("All active WebSocket requests completed. Proceeding with shutdown...")
	case <-ctx.Done():
		fmt.Println("[WARNING] Timeout waiting for WebSocket requests. Forcing shutdown...")
	}

	if err := srv.Shutdown(ctx); err != nil {
		fmt.Printf("Error shutting down JSON-RPC WebSocket server: %v\n", err)
	} else {
		fmt.Println("JSON-RPC WebSocket server stopped.")
	}
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		http.Error(w, "WebSocket upgrade failed", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	fmt.Println("New WebSocket connection established.")
	atomic.AddInt32(&activeJsonRPCWSRequestCount, 1)
	wg.Add(1)
	defer func() {
		wg.Done()
		atomic.AddInt32(&activeJsonRPCWSRequestCount, -1)
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure, 1005) {
				log.Printf("WebSocket closed by client: %v", err)
			} else {
				log.Printf("Error reading WebSocket message: %v", err)
			}
			break
		}

		fmt.Printf("Received raw message: %s\n", string(message))

		var req JSONRPCRequest
		err = json.Unmarshal(message, &req)
		if err != nil {
			log.Printf("Invalid JSON-RPC WebSocket request: %v", err)
			continue
		}

		fmt.Printf("Received JSON-RPC WS request: Method=%s, Params=%v, ID=%d\n", req.Method, req.Params, req.ID)

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			Result:  fmt.Sprintf("Method %s executed successfully", req.Method),
			ID:      req.ID,
		}

		respJSON, _ := json.Marshal(resp)
		if err := conn.WriteMessage(websocket.TextMessage, respJSON); err != nil {
			log.Printf("Error writing WebSocket response: %v", err)
			break
		}
	}
}
