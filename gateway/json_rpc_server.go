package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/decentrio/gateway/config"
	httpUtils "github.com/decentrio/gateway/utils"
)

// Error type
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC request format
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// JSON-RPC response format
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

var (
	jsonRPCServers            = make(map[uint16]*http.Server)
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
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	select {
	case semaphore <- struct{}{}:
		defer func() { <-semaphore }()
	case <-ctx.Done():
		http.Error(w, "Server busy, please try again later", http.StatusTooManyRequests)
		return
	}
	var req JSONRPCRequest
	var res JSONRPCResponse
	if r.Method != http.MethodPost {
		res = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Invalid request"},
			ID:      1,
		}
		json.NewEncoder(w).Encode(res)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		res = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
			ID:      1,
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = int64(len(body))

	err = json.Unmarshal(body, &req)
	if err != nil {
		res = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Invalid JSON-RPC request: " + err.Error()},
			ID:      -32700,
		}
		json.NewEncoder(w).Encode(res)
		return
	}

	fmt.Printf("Received JSON-RPC request: Method=%s, Params=%v\n", req.Method, req.Params)
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
		checkRequestManually(w, r)
		return
	case "eth_newFilter", /// ????
		"eth_getLogs":
		res = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Method not supported yet"},
			ID:      1,
		}
		json.NewEncoder(w).Encode(res)
		return
	case "eth_getBalance", // param 1
		"eth_getTransactionCount",
		"eth_getCode",
		"eth_call":
		height, err = getHeightFromParams(paramsMap, 1)
		if err != nil {
			res = JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      1,
			}
			json.NewEncoder(w).Encode(res)
			return
		}
	case "eth_getStorageAt": // param 2
		height, err = getHeightFromParams(paramsMap, 2)
		if err != nil {
			res = JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      1,
			}
			json.NewEncoder(w).Encode(res)
			return
		}
	case "eth_getBlockTransactionCountByNumber", // param 0
		"eth_getBlockByNumber",
		"eth_getTransactionByBlockNumberAndIndex",
		"eth_getUncleByBlockNumberAndIndex":
		height, err = getHeightFromParams(paramsMap, 0)
		if err != nil {
			res = JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      1,
			}
			json.NewEncoder(w).Encode(res)
			return
		}
	default:
		height = 0
	}

	fmt.Printf("Height: %d\n", height)
	node := config.GetNodebyHeight(height)
	if node == nil {
		res = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32602, Message: "No nodes found"},
			ID:      1,
		}

		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Println("Node called:", node.JSONRPC)
	httpUtils.FowardRequest(w, r, node.JSONRPC)
}

func getHeightFromParams(params []any, index int) (uint64, error) {
	if len(params) > index {
		height, found := params[index].(string)
		if found {
			if height == "latest" || height == "pending" {
				return 0, nil
			} else if height == "earliest" {
				return 1, nil // temporary, should be earliest possible
			} else if strings.HasPrefix(height, "0x") {
				height = strings.TrimPrefix(height, "0x")
				if h, err := strconv.ParseUint(height, 16, 64); err == nil {
					return h, nil
				} else {
					return math.MaxUint64, fmt.Errorf("invalid height parameter: %w", err)
				}
			} else {
				return math.MaxUint64, fmt.Errorf("invalid height parameter")
			}
		} else {
			return math.MaxUint64, fmt.Errorf("height not found")
		}
	} else {
		return math.MaxUint64, fmt.Errorf("invalid params")
	}
}

func checkRequestManually(w http.ResponseWriter, r *http.Request) {
	ETH_nodes := config.GetNodesByType("jsonrpc")
	var msg JSONRPCResponse

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		msg = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
			ID:      1,
		}
		json.NewEncoder(w).Encode(msg)
		return
	}

	for _, url := range ETH_nodes {
		new_r := r.Clone(r.Context())
		new_r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		res, err := httpUtils.CheckRequest(new_r, url)
		if err != nil || res == nil {
			continue
		}

		fmt.Println("Node called:", url)
		if res.Body != nil {
			body, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				msg = JSONRPCResponse{
					JSONRPC: "2.0",
					Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
					ID:      1,
				}
				json.NewEncoder(w).Encode(msg)
				return
			}

			json.Unmarshal(body, &msg)
		}

		if msg.Error != nil || msg.Result != nil {
			json.NewEncoder(w).Encode(msg)
			return
		} else if msg.Result == nil {
			fmt.Println("Result is empty")
			continue
		}
	}

	if msg.Result == nil {
		nil_msg := map[string]interface{}{
			"jsonrpc": msg.JSONRPC,
			"id":      msg.ID,
			"result":  msg.Result,
		}
		json.NewEncoder(w).Encode(nil_msg)
	}
}
