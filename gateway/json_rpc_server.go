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
	enableBatchRequests       bool // Flag to enable/disable batch request processing
	disableNotifications      bool // Flag to disable notification support (default missing id to 1)
	enableDebug               bool // Flag to enable debug_* methods (gated in config)
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// writeJSONRPCResponse sets Content-Type and encodes a single JSON-RPC response.
func writeJSONRPCResponse(w http.ResponseWriter, res JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// writeJSONRPCResponses sets Content-Type and encodes a batch of JSON-RPC responses.
func writeJSONRPCResponses(w http.ResponseWriter, responses []JSONRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responses)
}

// writeJSONBytes sets Content-Type and writes raw JSON bytes (e.g. [] for empty batch).
func writeJSONBytes(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// normalizeJSONRPCVersion defaults jsonrpc to "2.0" if it's missing or empty
func normalizeJSONRPCVersion(req *JSONRPCRequest) {
	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
}

// normalizeRequestID defaults id to "1" if it's missing and notifications are disabled
func normalizeRequestID(req *JSONRPCRequest) {
	if disableNotifications && len(req.ID) == 0 {
		fmt.Println("Adding ID to notification request")
		req.ID = json.RawMessage("1")
	}
}

// SetEnableBatchRequests sets whether batch requests should be enabled
func SetEnableBatchRequests(enabled bool) {
	enableBatchRequests = enabled
}

// SetDisableNotifications sets whether notification support should be disabled
// When disabled, requests without an 'id' field will default to id=1 and always receive a response
func SetDisableNotifications(disabled bool) {
	disableNotifications = disabled
}

// SetEnableDebug sets whether debug_* methods (gated in config) are supported
func SetEnableDebug(enabled bool) {
	enableDebug = enabled
}

func Start_JSON_RPC_Server(server *Server) {
	fmt.Printf("Starting JSON-RPC server on port %d\n", server.Port)
	if enableBatchRequests {
		fmt.Println("Batch requests are ENABLED")
	} else {
		fmt.Println("Batch requests are DISABLED")
	}

	if enableDebug {
		fmt.Println("debug_* methods are ENABLED")
	} else {
		fmt.Println("debug_* methods are DISABLED")
	}

	if disableNotifications {
		fmt.Println("Notifications are DISABLED, IDs will be set to 1")
	}

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

	if r.Method != http.MethodPost {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Invalid request"},
			ID:      cloneRawMessage(nullJSONRPCID),
		})
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
			ID:      cloneRawMessage(nullJSONRPCID),
		})
		return
	}

	// Check if the request is a batch (array) or single request (object)
	var firstChar byte
	for i := range body {
		if body[i] != ' ' && body[i] != '\t' && body[i] != '\n' && body[i] != '\r' {
			firstChar = body[i]
			break
		}
	}

	// If it starts with '[', it's a batch request
	// Only process batch requests if enabled
	if firstChar == '[' {
		if !enableBatchRequests {
			writeJSONRPCResponse(w, JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: "Batch requests are not enabled"},
				ID:      cloneRawMessage(nullJSONRPCID),
			})
			return
		}
		handleBatchRequest(w, r, body)
		return
	}

	// Otherwise, it's a single request
	handleSingleRequest(w, r, body)
}

func handleBatchRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	// Trim whitespace from body to ensure clean parsing
	body = bytes.TrimSpace(body)

	// Check for and replace smart quotes that might cause parsing issues
	// Replace common smart quote characters with regular quotes
	body = bytes.ReplaceAll(body, []byte("\u201C"), []byte("\"")) // Left double quotation mark
	body = bytes.ReplaceAll(body, []byte("\u201D"), []byte("\"")) // Right double quotation mark
	body = bytes.ReplaceAll(body, []byte("\u2018"), []byte("'"))  // Left single quotation mark
	body = bytes.ReplaceAll(body, []byte("\u2019"), []byte("'"))  // Right single quotation mark

	var requests []JSONRPCRequest
	err := json.Unmarshal(body, &requests)
	if err != nil {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Invalid JSON-RPC batch request: " + err.Error()},
			ID:      cloneRawMessage(nullJSONRPCID),
		})
		return
	}

	// Empty batch array - per JSON-RPC 2.0 spec, return empty array
	if len(requests) == 0 {
		writeJSONBytes(w, []byte("[]"))
		return
	}

	// Normalize jsonrpc version for each request (default to "2.0" if missing)
	// Also normalize request ID if notifications are disabled
	for i := range requests {
		normalizeJSONRPCVersion(&requests[i])
		normalizeRequestID(&requests[i])
	}

	// Process each request in the batch
	responses := make([]JSONRPCResponse, 0, len(requests))
	for _, req := range requests {
		res := processSingleJSONRPCRequestForBatchRequest(r, req)
		// Only include responses for non-notification requests (those with an ID)
		if len(req.ID) > 0 && string(req.ID) != "null" {
			responses = append(responses, res)
		}
	}

	// If all requests were notifications, return empty array
	// Otherwise, return array of responses
	if len(responses) == 0 {
		writeJSONBytes(w, []byte("[]"))
		return
	}

	writeJSONRPCResponses(w, responses)
}

func handleSingleRequest(w http.ResponseWriter, r *http.Request, body []byte) {
	var req JSONRPCRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Invalid JSON-RPC request: " + err.Error()},
			ID:      cloneRawMessage(nullJSONRPCID),
		})
		return
	}

	// Normalize jsonrpc version (default to "2.0" if missing)
	normalizeJSONRPCVersion(&req)
	// Normalize request ID if notifications are disabled
	normalizeRequestID(&req)

	// Re-marshal the normalized request to ensure the body includes any normalized fields
	normalizedBody, err := json.Marshal(req)
	if err != nil {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Failed to marshal normalized request: " + err.Error()},
			ID:      ensureResponseID(req.ID),
		})
		return
	}

	// Use normalized body for forwarding
	r.Body = io.NopCloser(bytes.NewReader(normalizedBody))
	r.Header.Set("Content-Type", "application/json")
	r.ContentLength = int64(len(normalizedBody))

	// Use original single-request logic (forwards directly to ResponseWriter)
	processSingleJSONRPCRequest(w, r, req, normalizedBody)
}

// isHashBasedMethod checks if a method requires hash-based manual checking (from config).
func isHashBasedMethod(method string) bool {
	return config.GetMethodRouting(method).HashBased
}

// getHeightForMethod extracts height from params based on method routing config.
// Returns (height, error). If height is math.MaxUint64, it means no height was determined.
// Gated methods when flag is off return (0, nil) so the core returns "Method not supported".
func getHeightForMethod(method string, paramsMap []any) (uint64, error) {
	routing := config.GetMethodRouting(method)
	if routing.HeightFromParam == nil {
		return 0, nil
	}
	if routing.Gated && !enableDebug {
		return 0, nil
	}
	return getHeightFromParams(paramsMap, *routing.HeightFromParam)
}

// handleRequestWithManualCheckCore is the core logic for manually checking requests across multiple nodes
// It tries each node until it finds a successful response
func handleRequestWithManualCheckCore(r *http.Request, req JSONRPCRequest) JSONRPCResponse {
	ETH_nodes := config.GetNodesByType("jsonrpc")
	var msg JSONRPCResponse

	// Create a request body with just this single request
	reqBody, _ := json.Marshal(req)
	newReq := r.Clone(r.Context())
	newReq.Body = io.NopCloser(bytes.NewReader(reqBody))
	newReq.Header.Set("Content-Type", "application/json")
	newReq.ContentLength = int64(len(reqBody))

	for _, url := range ETH_nodes {
		msg = JSONRPCResponse{}
		testReq := newReq.Clone(r.Context())
		testReq.Body = io.NopCloser(bytes.NewReader(reqBody))
		res, err := httpUtils.CheckRequest(testReq, url)
		if err != nil || res == nil {
			fmt.Printf("Node %s: request failed (err=%v, res=%v)\n", url, err, res != nil)
			continue
		}

		var body []byte
		if res.Body != nil {
			body, err = io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				if errors.Is(err, context.Canceled) {
					fmt.Printf("Node %s: response read failed (client disconnected or request canceled)\n", url)
				} else {
					fmt.Printf("Node %s: failed to read body: %v\n", url, err)
				}
				continue
			}
			json.Unmarshal(body, &msg)
		}
		if msg.Error == nil && msg.Result != nil {
			msg.ID = ensureResponseID(req.ID)
			return msg
		}
		// Log why we're moving to the next node (result empty or error set)
		if msg.Error != nil {
			fmt.Printf("Node %s: JSON-RPC error: code=%d message=%s\n", url, msg.Error.Code, msg.Error.Message)
		} else {
			fmt.Printf("Node %s: result is empty. response body (first 500 chars): %s\n", url, truncate(string(body), 500))
		}
		continue
	}

	// No successful response found
	return JSONRPCResponse{
		JSONRPC: "2.0",
		Error:   &JSONRPCError{Code: -32603, Message: "Internal error"},
		ID:      ensureResponseID(req.ID),
	}
}

// processSingleJSONRPCRequestCore contains the core logic for processing a single JSON-RPC request.
// It returns a JSONRPCResponse that can be used for both single and batch requests.
// Routing is driven by config method_routing (see config.GetMethodRouting).
func processSingleJSONRPCRequestCore(r *http.Request, req JSONRPCRequest, body []byte) JSONRPCResponse {
	fmt.Printf("Received JSON-RPC request: Method=%s, ID=%s, Params=%s\n", req.Method, formatIDForLog(req.ID), string(req.Params))
	var paramsMap []any
	json.Unmarshal(req.Params, &paramsMap)
	var height uint64 = math.MaxUint64
	var err error

	routing := config.GetMethodRouting(req.Method)

	if routing.NotSupported {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Method not supported"},
			ID:      ensureResponseID(req.ID),
		}
	}
	if routing.Gated && !enableDebug {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Method not supported"},
			ID:      ensureResponseID(req.ID),
		}
	}
	if routing.HashBased {
		return handleRequestWithManualCheckCore(r, req)
	}
	if routing.HeightFromParam != nil {
		height, err = getHeightFromParams(paramsMap, *routing.HeightFromParam)
		if err != nil {
			if errors.Is(err, errBlockHashSelector) {
				return handleRequestWithManualCheckCore(r, req)
			}
			return JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      ensureResponseID(req.ID),
			}
		}
	} else {
		height = 0
	}

	fmt.Printf("Height: %d\n", height)
	node := config.GetNodebyHeight(height)
	if node == nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32602, Message: "No nodes found"},
			ID:      ensureResponseID(req.ID),
		}
	}
	fmt.Println("Node called:", node.JSONRPC)

	// Create a new request with just this single request
	reqBody, _ := json.Marshal(req)
	newReq := r.Clone(r.Context())
	newReq.Body = io.NopCloser(bytes.NewReader(reqBody))
	newReq.Header.Set("Content-Type", "application/json")
	newReq.ContentLength = int64(len(reqBody))

	// Forward the request and get the response
	res, err := forwardRequestAndGetResponse(newReq, node.JSONRPC)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32603, Message: "Internal error: " + err.Error()},
			ID:      ensureResponseID(req.ID),
		}
	}

	res.ID = ensureResponseID(req.ID)
	return res
}

// getHeightFromEthGetLogsFilter extracts height from eth_getLogs filter object
// Returns (height, error). If blockHash is present, returns errBlockHashSelector.
func getHeightFromEthGetLogsFilter(paramsMap []any) (uint64, error) {
	if len(paramsMap) == 0 || paramsMap[0] == nil {
		// No filter provided, default to latest
		return 0, nil
	}

	filter, ok := paramsMap[0].(map[string]any)
	if !ok {
		return math.MaxUint64, fmt.Errorf("invalid filter parameter")
	}

	// If blockHash is present, use manual checking
	if blockHash, ok := filter["blockHash"]; ok {
		if _, ok := blockHash.(string); ok {
			return 0, errBlockHashSelector
		}
		return math.MaxUint64, fmt.Errorf("invalid blockHash parameter")
	}

	// Try fromBlock first, then toBlock as fallback
	if fromBlock, ok := filter["fromBlock"]; ok {
		return parseHeightFromAny(fromBlock)
	}

	if toBlock, ok := filter["toBlock"]; ok {
		return parseHeightFromAny(toBlock)
	}

	// No block info provided, default to latest
	return 0, nil
}

// ethGetLogsResult is the result of handling an eth_getLogs request.
// Exactly one of NeedManualCheck, Response (error), or Node (forward) is used.
type ethGetLogsResult struct {
	NeedManualCheck bool
	Response        *JSONRPCResponse
	Node            *config.Node
}

// handleEthGetLogsRequest resolves routing for eth_getLogs from its filter params.
// Caller should: if NeedManualCheck do manual check; else if Response != nil send it; else forward to Node.
func handleEthGetLogsRequest(req JSONRPCRequest, paramsMap []any) ethGetLogsResult {
	height, err := getHeightFromEthGetLogsFilter(paramsMap)
	if err != nil && errors.Is(err, errBlockHashSelector) {
		return ethGetLogsResult{NeedManualCheck: true}
	}
	if err != nil {
		return ethGetLogsResult{
			Response: &JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32600, Message: err.Error()},
				ID:      ensureResponseID(req.ID),
			},
		}
	}
	node := config.GetNodebyHeight(height)
	if node == nil {
		return ethGetLogsResult{
			Response: &JSONRPCResponse{
				JSONRPC: "2.0",
				Error:   &JSONRPCError{Code: -32602, Message: "No nodes found"},
				ID:      ensureResponseID(req.ID),
			},
		}
	}
	return ethGetLogsResult{Node: node}
}

// processSingleJSONRPCRequest processes a single request and writes the response to ResponseWriter
func processSingleJSONRPCRequest(w http.ResponseWriter, r *http.Request, req JSONRPCRequest, body []byte) {
	if req.Method == "eth_getLogs" {
		var paramsMap []any
		json.Unmarshal(req.Params, &paramsMap)
		result := handleEthGetLogsRequest(req, paramsMap)
		if result.NeedManualCheck {
			writeJSONRPCResponse(w, handleRequestWithManualCheckCore(r, req))
			return
		}
		if result.Response != nil {
			writeJSONRPCResponse(w, *result.Response)
			return
		}
		if result.Node != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.ContentLength = int64(len(body))
			httpUtils.FowardRequest(w, r, result.Node.JSONRPC)
			return
		}
		return
	}

	// Check if this is a hash-based method that needs manual checking
	needsManualCheck := isHashBasedMethod(req.Method)

	// Check if we can determine height and forward directly
	var paramsMap []any
	json.Unmarshal(req.Params, &paramsMap)
	height, heightErr := getHeightForMethod(req.Method, paramsMap)

	// If it's a hash-based method or has a block hash selector error, use manual check
	if needsManualCheck || (heightErr != nil && errors.Is(heightErr, errBlockHashSelector)) {
		writeJSONRPCResponse(w, handleRequestWithManualCheckCore(r, req))
		return
	}

	// For height-based methods, try to forward directly if possible (performance optimization)
	if heightErr == nil && height != math.MaxUint64 {
		node := config.GetNodebyHeight(height)
		if node != nil {
			// Restore body for forwarding
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.ContentLength = int64(len(body))
			httpUtils.FowardRequest(w, r, node.JSONRPC)
			return
		}
	}

	// Otherwise, use the core function
	writeJSONRPCResponse(w, processSingleJSONRPCRequestCore(r, req, body))
}

// processSingleJSONRPCRequestForBatchRequest processes a single request within a batch request
func processSingleJSONRPCRequestForBatchRequest(r *http.Request, req JSONRPCRequest) JSONRPCResponse {
	reqBody, _ := json.Marshal(req)
	return processSingleJSONRPCRequestCore(r, req, reqBody)
}

// forwardRequestAndGetResponse forwards a request and returns the JSON-RPC response
func forwardRequestAndGetResponse(r *http.Request, destination string) (JSONRPCResponse, error) {
	res, err := httpUtils.CheckRequest(r, destination)
	if err != nil {
		return JSONRPCResponse{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return JSONRPCResponse{}, err
	}

	var jsonRes JSONRPCResponse
	if err := json.Unmarshal(body, &jsonRes); err != nil {
		return JSONRPCResponse{}, err
	}

	return jsonRes, nil
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
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
			ID:      cloneRawMessage(nullJSONRPCID),
		})
		return
	}

	// Parse the request from the body
	var req JSONRPCRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeJSONRPCResponse(w, JSONRPCResponse{
			JSONRPC: "2.0",
			Error:   &JSONRPCError{Code: -32600, Message: "Parse error. Invalid JSON: " + err.Error()},
			ID:      cloneRawMessage(nullJSONRPCID),
		})
		return
	}

	// Use the unified core function
	writeJSONRPCResponse(w, handleRequestWithManualCheckCore(r, req))
}
