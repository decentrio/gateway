package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		log.Printf("WebSocket request from: %s", r.Host)
		return true
	},
}

func Start_JSON_RPC_WS_Server(server *Server) {
	mux := http.NewServeMux()
	mux.HandleFunc("/websocket", handleWebSocket)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", server.Port),
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Error starting JSON-RPC WebSocket server: %v", err)
		}
	}()

	fmt.Printf("JSON-RPC WebSocket server is running on port %d\n", server.Port)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	fmt.Println("\nShutting down JSON-RPC WebSocket server...")
	if err := srv.Close(); err != nil {
		log.Fatalf("Error shutting down JSON-RPC WebSocket server: %v", err)
	}
	fmt.Println("JSON-RPC WebSocket server stopped.")
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

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			// Nếu client đóng kết nối, chỉ log mà không gửi close message
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

func Shutdown_JSON_RPC_WS_Server(server *Server) {
	fmt.Println("Shutting down WebSocket server")
	os.Exit(0)
}
