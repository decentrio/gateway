package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/decentrio/gateway/config"
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

func Shutdown_JSON_RPC_WS_Server(server *Server) {
	fmt.Println("Shutting down WebSocket server")
	os.Exit(0)
}

func isWebSocketAvailable(wsURL string) bool {
	wsURL = strings.TrimPrefix(wsURL, "ws://")
	wsURL = strings.TrimPrefix(wsURL, "wss://")

	hostPort := strings.Split(wsURL, "/")[0] 

	conn, err := net.DialTimeout("tcp", hostPort, 2*time.Second)
	if err != nil {
		log.Printf("WebSocket %s is not available: %v", wsURL, err)
		return false
	}
	conn.Close()
	return true
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	var node *config.Node
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

		var height uint64
		if params, ok := req.Params.(map[string]interface{}); ok {
			if h, ok := params["height"].(float64); ok {
				height = uint64(h)
			}
		}

		if height > 0 {
			node = config.GetNodebyHeight(height)
			if node == nil {
				respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"Node not found","id":%d}`, req.ID)
				conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
				continue
			}
		}

		if node != nil {
			fmt.Printf("Forwarding to Node: ", node.JSONRPC_WS)

			if !isWebSocketAvailable(node.JSONRPC_WS) {
				log.Printf("WebSocket unavailable: %s", node.JSONRPC_WS)
				respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"WebSocket node unavailable","id":%d}`, req.ID)
				conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
				continue
			}

			dialURL := strings.TrimPrefix(node.JSONRPC_WS, "ws://")
			dialURL = strings.TrimPrefix(dialURL, "wss://")
			hostPort := strings.Split(dialURL, "/")[0] 

			nodeConn, _, err := websocket.DefaultDialer.Dial("ws://"+hostPort+"/websocket", nil)

			if err != nil {
				log.Printf("Failed to connect to RPC WebSocket: %v", err)
				respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"Failed to connect to RPC WebSocket","id":%d}`, req.ID)
				conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
				continue
			}
			defer nodeConn.Close()
		}

	}
}
