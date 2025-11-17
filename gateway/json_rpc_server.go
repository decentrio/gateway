package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// JSON-RPC response format
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

var (
	jsonRPCServers            = make(map[uint16]*http.Server)
	activeJsonRPCRequestCount int32
)

var errBlockHashSelector = errors.New("block hash selector provided")
var nullJSONRPCID = json.RawMessage("null")

func cloneRawMessage(id json.RawMessage) json.RawMessage {
	if id == nil {
		return nil
	}
	cloned := make([]byte, len(id))
	copy(cloned, id)
	return cloned
}

func ensureResponseID(id json.RawMessage) json.RawMessage {
	if len(id) == 0 {
		return cloneRawMessage(nullJSONRPCID)
	}
	return cloneRawMessage(id)
}

func formatIDForLog(id json.RawMessage) string {
	if len(id) == 0 {
		return "null"
	}
	return string(id)
}

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
			ID:      cloneRawMessage(nullJSONRPCID),
		}
		json.NewEncoder(w).Encode(res)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		res = JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
			ID:      cloneRawMessage(nullJSONRPCID),
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
			ID:      cloneRawMessage(nullJSONRPCID),
		}
		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Printf("Received JSON-RPC request: Method=%s, ID=%s, Params=%s\n", req.Method, formatIDForLog(req.ID), string(req.Params))
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
			ID:      ensureResponseID(req.ID),
		}
		json.NewEncoder(w).Encode(res)
		return
	case "eth_getBalance", // param 1
		"eth_getTransactionCount",
		"eth_getCode",
		"eth_call":
		height, err = getHeightFromParams(paramsMap, 1)
		if err != nil {
			if errors.Is(err, errBlockHashSelector) {
				r.Body = io.NopCloser(bytes.NewReader(body))
				checkRequestManually(w, r)
				return
			}
			res = JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      ensureResponseID(req.ID),
			}
			json.NewEncoder(w).Encode(res)
			return
		}
	case "eth_getStorageAt": // param 2
		height, err = getHeightFromParams(paramsMap, 2)
		if err != nil {
			if errors.Is(err, errBlockHashSelector) {
				r.Body = io.NopCloser(bytes.NewReader(body))
				checkRequestManually(w, r)
				return
			}
			res = JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      ensureResponseID(req.ID),
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
			if errors.Is(err, errBlockHashSelector) {
				r.Body = io.NopCloser(bytes.NewReader(body))
				checkRequestManually(w, r)
				return
			}
			res = JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      ensureResponseID(req.ID),
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
			ID:      ensureResponseID(req.ID),
		}

		json.NewEncoder(w).Encode(res)
		return
	}
	fmt.Println("Node called:", node.JSONRPC)
	httpUtils.FowardRequest(w, r, node.JSONRPC)
}

func getHeightFromParams(params []any, index int) (uint64, error) {
	if len(params) <= index || params[index] == nil {
		// Missing or null height selector defaults to latest.
		return 0, nil
	}

	switch v := params[index].(type) {
	case string:
		return parseHeightSelector(v)
	case map[string]any:
		return parseHeightFromSelectorObject(v)
	default:
		return math.MaxUint64, fmt.Errorf("height not found")
	}
}

func parseHeightSelector(value string) (uint64, error) {
	switch value {
	case "latest", "pending":
		return 0, nil
	case "earliest":
		return 1, nil // temporary, should be earliest possible
	}

	if strings.HasPrefix(value, "0x") {
		value = strings.TrimPrefix(value, "0x")
		if h, err := strconv.ParseUint(value, 16, 64); err == nil {
			return h, nil
		} else {
			return math.MaxUint64, fmt.Errorf("invalid height parameter: %w", err)
		}
	}

	if parsed, err := strconv.ParseUint(value, 10, 64); err == nil {
		return parsed, nil
	}

	return math.MaxUint64, fmt.Errorf("invalid height parameter")
}

func parseHeightFromSelectorObject(selector map[string]any) (uint64, error) {
	if blockNumber, ok := selector["blockNumber"]; ok {
		return parseHeightFromAny(blockNumber)
	}

	if blockHash, ok := selector["blockHash"]; ok {
		if _, ok := blockHash.(string); ok {
			return 0, errBlockHashSelector
		}
		return math.MaxUint64, fmt.Errorf("invalid blockHash parameter")
	}

	if blockTag, ok := selector["blockTag"]; ok {
		return parseHeightFromAny(blockTag)
	}

	return math.MaxUint64, fmt.Errorf("height not found")
}

func parseHeightFromAny(value any) (uint64, error) {
	switch v := value.(type) {
	case string:
		return parseHeightSelector(v)
	case float64:
		return uint64(v), nil
	case map[string]any:
		return parseHeightFromSelectorObject(v)
	default:
		return math.MaxUint64, fmt.Errorf("invalid height parameter")
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
			ID:      cloneRawMessage(nullJSONRPCID),
		}
		json.NewEncoder(w).Encode(msg)
		return
	}

	for _, url := range ETH_nodes {
		msg = JSONRPCResponse{}
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
					ID:      cloneRawMessage(nullJSONRPCID),
				}
				json.NewEncoder(w).Encode(msg)
				return
			}

			json.Unmarshal(body, &msg)
		}

		if msg.Error == nil && msg.Result != nil {
			msg.ID = ensureResponseID(msg.ID)
			json.NewEncoder(w).Encode(msg)
			return
		} else if msg.Result == nil {
			fmt.Println("Result is empty")
			continue
		}
	}

	if msg.Result == nil {
		response := JSONRPCResponse{
			JSONRPC: msg.JSONRPC,
			ID:      ensureResponseID(msg.ID),
			Result:  msg.Result,
		}
		json.NewEncoder(w).Encode(response)
	}
}
