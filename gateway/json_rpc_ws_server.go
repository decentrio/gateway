package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/decentrio/gateway/config"
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

	fmt.Printf("Waiting for %d requests to complete before shutting down JSON-RPC-WebSocket server ...\n", atomic.LoadInt32(&activeJsonRPCWSRequestCount))

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

		var req JSONRPCRequest
		err = json.Unmarshal(message, &req)
		if err != nil {
			log.Printf("Invalid JSON-RPC WebSocket request: %v", err)
			continue
		}

		fmt.Printf("Received JSON-RPC WS request: Method=%s, Params=%v, ID=%d\n", req.Method, req.Params, req.ID)

		paramsMap := make([]any, len(req.Params))
		json.Unmarshal(req.Params, &paramsMap)
		var height uint64 = math.MaxUint64

		switch req.Method {
		case "eth_getTransactionByHash", // tx hash in params
			"eth_getTransactionReceipt",
			"eth_getBlockByHash", // block hash in params
			"eth_getBlockTransactionCountByHash",
			"eth_getTransactionByBlockHashAndIndex",
			"eth_getUncleByBlockHashAndIndex":
			checkRequestManuallyWebSocket(conn, req)
			continue

		case "eth_newFilter", "eth_getLogs":
			respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"Method not supported yet","id":%d}`, req.ID)
			conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
			continue

		case "eth_getBalance", "eth_getTransactionCount", "eth_getCode", "eth_call":
			height, err = getHeightFromParams(paramsMap, 1)
		case "eth_getStorageAt":
			height, err = getHeightFromParams(paramsMap, 2)
		case "eth_getBlockTransactionCountByNumber", "eth_getBlockByNumber",
			"eth_getTransactionByBlockNumberAndIndex", "eth_getUncleByBlockNumberAndIndex":
			height, err = getHeightFromParams(paramsMap, 0)
		default:
			height = 0
		}

		if err != nil {
			respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"%s","id":%d}`, err.Error(), req.ID)
			conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
			continue
		}
		if node == nil {
			if defaultNode := config.GetNodebyHeight(0); defaultNode != nil {
				node = defaultNode
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
		fmt.Printf("Height: %d\n", height)

		if node != nil {
			fmt.Printf("Forwarding to Node: %s\n", node.JSONRPC_WS)

			if !isWebSocketAvailable(node.JSONRPC_WS) {
				log.Printf("WebSocket unavailable: %s", node.JSONRPC_WS)
				respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"WebSocket node unavailable","id":%d}`, req.ID)
				conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
				continue
			}
			dialURL := strings.TrimPrefix(node.JSONRPC_WS, "ws://")
			dialURL = strings.TrimPrefix(dialURL, "wss://")
			hostPort := strings.Split(dialURL, "/")[0]

			nodeConn, _, err := websocket.DefaultDialer.Dial("ws://"+hostPort, nil)
			// nodeConn, _, err := websocket.DefaultDialer.Dial(node.JSONRPC_WS, nil)
			if err != nil {
				log.Printf("Failed to connect to jsonRPC WebSocket: %v", err)
				respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"Failed to connect to jsonRPC WebSocket","id":%d}`, req.ID)
				conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
				continue
			}
			defer nodeConn.Close()

			err = nodeConn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Error forwarding message to node: %v", err)
				continue
			}

			_, response, err := nodeConn.ReadMessage()
			if err != nil {
				log.Printf("Error reading response from node: %v", err)
				respJSON := fmt.Sprintf(`{"jsonrpc":"2.0","error":"Failed to read response from node","id":%d}`, req.ID)
				conn.WriteMessage(websocket.TextMessage, []byte(respJSON))
				continue
			}

			// fmt.Printf("Received response from node: %s\n", string(response))

			err = conn.WriteMessage(websocket.TextMessage, response)
			if err != nil {
				log.Printf("Error sending response back to client: %v", err)
			}
		}
	}
}

func checkRequestManuallyWebSocket(conn *websocket.Conn, request JSONRPCRequest) {
	ETH_nodes := config.GetNodesByType("jsonrpc_ws")
	var wg sync.WaitGroup
	var bestNode atomic.Value
	responseChan := make(chan map[string]interface{}, len(ETH_nodes))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, url := range ETH_nodes {
		wg.Add(1)
		go func(nodeURL string) {
			defer wg.Done()

			ws, _, err := websocket.DefaultDialer.Dial(nodeURL, nil)
			if err != nil {
				log.Printf("Failed to connect to node %s: %v", nodeURL, err)
				return
			}
			defer ws.Close()

			err = ws.WriteJSON(request)
			if err != nil {
				log.Printf("Failed to send request to node %s: %v", nodeURL, err)
				return
			}

			var res map[string]interface{}
			err = ws.ReadJSON(&res)
			if err != nil {
				log.Printf("Failed to read response from node %s: %v", nodeURL, err)
				return
			}

			if result, ok := res["result"]; ok && result != nil {
				bestNode.Store(nodeURL)
				responseChan <- res
			} else {
				log.Printf("Node %s responded but has no valid result", nodeURL)
			}
		}(url)
	}

	go func() {
		wg.Wait()
		close(responseChan)
	}()

	var bestResponse map[string]interface{}
	select {
	case bestResponse = <-responseChan:
		if nodeURL, ok := bestNode.Load().(string); ok {
			fmt.Println("Node called:", nodeURL)
		}
	case <-ctx.Done():
		log.Println("Timeout: No valid response from nodes")
		bestResponse = map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      request.ID,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": "No valid response from nodes",
			},
		}
	}

	if err := conn.WriteJSON(bestResponse); err != nil {
		log.Println("Failed to send response to client:", err)
	}
}
