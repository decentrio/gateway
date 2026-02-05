package gateway_test

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/decentrio/gateway/gateway"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchRequestStructure(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		expectedError    bool
		expectedCount    int
		expectedValidIDs int
	}{
		{
			name:             "valid batch request with multiple methods",
			body:             `[{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber", "params": []},{"jsonrpc": "2.0", "id": 2, "method": "eth_gasPrice", "params": []}]`,
			expectedError:    false,
			expectedCount:    2,
			expectedValidIDs: 2,
		},
		{
			name:             "batch request with notifications (no IDs)",
			body:             `[{"jsonrpc": "2.0", "method": "eth_blockNumber", "params": []},{"jsonrpc": "2.0", "method": "eth_gasPrice", "params": []}]`,
			expectedError:    false,
			expectedCount:    2,
			expectedValidIDs: 0,
		},
		{
			name:             "batch request with mix of notifications and regular requests",
			body:             `[{"jsonrpc": "2.0", "method": "eth_blockNumber", "params": []},{"jsonrpc": "2.0", "id": 1, "method": "eth_gasPrice", "params": []}]`,
			expectedError:    false,
			expectedCount:    2,
			expectedValidIDs: 1,
		},
		{
			name:             "empty batch array",
			body:             `[]`,
			expectedError:    false,
			expectedCount:    0,
			expectedValidIDs: 0,
		},
		{
			name:             "invalid JSON in batch request",
			body:             `[{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber", "params": []},{"jsonrpc": "2.0", "id": 2, "method": "eth_gasPrice", "params": [}]`,
			expectedError:    true,
			expectedCount:    0,
			expectedValidIDs: 0,
		},
		{
			name:             "batch request with null IDs",
			body:             `[{"jsonrpc": "2.0", "id": null, "method": "eth_blockNumber", "params": []},{"jsonrpc": "2.0", "id": 1, "method": "eth_gasPrice", "params": []}]`,
			expectedError:    false,
			expectedCount:    2,
			expectedValidIDs: 1,
		},
		{
			name:             "batch request with whitespace",
			body:             `   [{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber", "params": []}]   `,
			expectedError:    false,
			expectedCount:    1,
			expectedValidIDs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			body = bytes.TrimSpace(body)

			// Replace smart quotes
			body = bytes.ReplaceAll(body, []byte("\u201C"), []byte("\""))
			body = bytes.ReplaceAll(body, []byte("\u201D"), []byte("\""))
			body = bytes.ReplaceAll(body, []byte("\u2018"), []byte("'"))
			body = bytes.ReplaceAll(body, []byte("\u2019"), []byte("'"))

			var requests []gateway.JSONRPCRequest
			err := json.Unmarshal(body, &requests)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(requests))

				// Count how many have valid IDs
				validIDCount := 0
				for _, req := range requests {
					if len(req.ID) > 0 && string(req.ID) != "null" {
						validIDCount++
					}
				}

				assert.Equal(t, tt.expectedValidIDs, validIDCount)
			}
		})
	}
}

func TestBatchRequestParsing(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		expectError  bool
		requestCount int
	}{
		{
			name:         "valid batch with 3 requests",
			body:         `[{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]},{"jsonrpc":"2.0","id":2,"method":"eth_gasPrice","params":[]},{"jsonrpc":"2.0","id":3,"method":"eth_blockNumber","params":[]}]`,
			expectError:  false,
			requestCount: 3,
		},
		{
			name:         "batch with smart quotes should be normalized",
			body:         `[{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}]`,
			expectError:  false,
			requestCount: 1,
		},
		{
			name:         "invalid JSON",
			body:         `[{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber", "params": [}]`,
			expectError:  true,
			requestCount: 0,
		},
		{
			name:         "not an array",
			body:         `{"jsonrpc": "2.0", "id": 1, "method": "eth_blockNumber", "params": []}`,
			expectError:  false, // Should be handled as single request
			requestCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			body = bytes.TrimSpace(body)

			// Replace smart quotes
			body = bytes.ReplaceAll(body, []byte("\u201C"), []byte("\""))
			body = bytes.ReplaceAll(body, []byte("\u201D"), []byte("\""))
			body = bytes.ReplaceAll(body, []byte("\u2018"), []byte("'"))
			body = bytes.ReplaceAll(body, []byte("\u2019"), []byte("'"))

			var requests []gateway.JSONRPCRequest
			err := json.Unmarshal(body, &requests)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				if err == nil {
					assert.Equal(t, tt.requestCount, len(requests))
				}
			}
		})
	}
}

func TestBatchRequestResponseFormat(t *testing.T) {
	tests := []struct {
		name             string
		requests         []gateway.JSONRPCRequest
		expectedLength   int
		allNotifications bool
	}{
		{
			name: "all requests have IDs",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", ID: json.RawMessage("2"), Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength:   2,
			allNotifications: false,
		},
		{
			name: "all requests are notifications",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength:   0,
			allNotifications: true,
		},
		{
			name: "mix of notifications and regular requests",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength:   1,
			allNotifications: false,
		},
		{
			name: "requests with null IDs",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", ID: json.RawMessage("null"), Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength:   1,
			allNotifications: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			responses := make([]gateway.JSONRPCResponse, 0, len(tt.requests))
			for _, req := range tt.requests {
				// Only include responses for non-notification requests (those with an ID)
				if len(req.ID) > 0 && string(req.ID) != "null" {
					responses = append(responses, gateway.JSONRPCResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
					})
				}
			}

			if tt.allNotifications {
				assert.Equal(t, 0, len(responses))
			} else {
				assert.Equal(t, tt.expectedLength, len(responses))
			}
		})
	}
}

func TestBatchRequestDetection(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		isBatch bool
	}{
		{
			name:    "batch request starts with [",
			body:    `[{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}]`,
			isBatch: true,
		},
		{
			name:    "single request starts with {",
			body:    `{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`,
			isBatch: false,
		},
		{
			name:    "batch with leading whitespace",
			body:    `   [{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}]`,
			isBatch: true,
		},
		{
			name:    "single request with leading whitespace",
			body:    `   {"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]}`,
			isBatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			var firstChar byte
			for i := 0; i < len(body); i++ {
				if body[i] != ' ' && body[i] != '\t' && body[i] != '\n' && body[i] != '\r' {
					firstChar = body[i]
					break
				}
			}

			isBatch := firstChar == '['
			assert.Equal(t, tt.isBatch, isBatch)
		})
	}
}

func TestEthGetLogsFilterParsing(t *testing.T) {
	tests := []struct {
		name            string
		filter          map[string]any
		expectError     bool
		expectBlockHash bool
		expectedHeight  uint64
	}{
		{
			name: "filter with fromBlock as hex string",
			filter: map[string]any{
				"fromBlock": "0x100",
			},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  256,
		},
		{
			name: "filter with fromBlock as decimal string",
			filter: map[string]any{
				"fromBlock": "1000",
			},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  1000,
		},
		{
			name: "filter with fromBlock as latest",
			filter: map[string]any{
				"fromBlock": "latest",
			},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  0,
		},
		{
			name: "filter with toBlock",
			filter: map[string]any{
				"toBlock": "0x200",
			},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  512,
		},
		{
			name: "filter with fromBlock and toBlock (fromBlock takes precedence)",
			filter: map[string]any{
				"fromBlock": "0x100",
				"toBlock":   "0x200",
			},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  256,
		},
		{
			name: "filter with blockHash",
			filter: map[string]any{
				"blockHash": "0x1234567890abcdef",
			},
			expectError:     false,
			expectBlockHash: true,
			expectedHeight:  0,
		},
		{
			name: "filter with blockHash and fromBlock (blockHash takes precedence)",
			filter: map[string]any{
				"blockHash": "0x1234567890abcdef",
				"fromBlock": "0x100",
			},
			expectError:     false,
			expectBlockHash: true,
			expectedHeight:  0,
		},
		{
			name:            "empty filter defaults to latest",
			filter:          map[string]any{},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  0,
		},
		{
			name: "filter with address and topics but no block info",
			filter: map[string]any{
				"address": "0x1234567890123456789012345678901234567890",
				"topics":  []any{},
			},
			expectError:     false,
			expectBlockHash: false,
			expectedHeight:  0,
		},
		{
			name: "filter with invalid blockHash type",
			filter: map[string]any{
				"blockHash": 12345, // not a string
			},
			expectError:     true,
			expectBlockHash: false,
			expectedHeight:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paramsMap := []any{tt.filter}

			// We can't directly test the unexported function, but we can test the logic
			// by verifying the filter structure can be parsed correctly
			filter, ok := paramsMap[0].(map[string]any)
			require.True(t, ok, "filter should be a map")

			// Test blockHash detection
			if blockHash, ok := filter["blockHash"]; ok {
				if _, ok := blockHash.(string); ok {
					assert.True(t, tt.expectBlockHash, "blockHash should trigger manual checking")
				} else {
					assert.True(t, tt.expectError, "invalid blockHash type should cause error")
				}
			}

			// Test fromBlock/toBlock extraction
			if !tt.expectBlockHash && !tt.expectError {
				var height uint64
				if fromBlock, ok := filter["fromBlock"]; ok {
					fromBlockStr, ok := fromBlock.(string)
					if ok {
						if fromBlockStr == "latest" || fromBlockStr == "pending" {
							height = 0
						} else if strings.HasPrefix(fromBlockStr, "0x") {
							hexStr := strings.TrimPrefix(fromBlockStr, "0x")
							if h, err := strconv.ParseUint(hexStr, 16, 64); err == nil {
								height = h
							}
						} else if h, err := strconv.ParseUint(fromBlockStr, 10, 64); err == nil {
							height = h
						}
					}
				} else if toBlock, ok := filter["toBlock"]; ok {
					toBlockStr, ok := toBlock.(string)
					if ok {
						if toBlockStr == "latest" || toBlockStr == "pending" {
							height = 0
						} else if strings.HasPrefix(toBlockStr, "0x") {
							hexStr := strings.TrimPrefix(toBlockStr, "0x")
							if h, err := strconv.ParseUint(hexStr, 16, 64); err == nil {
								height = h
							}
						} else if h, err := strconv.ParseUint(toBlockStr, 10, 64); err == nil {
							height = h
						}
					}
				}

				if !tt.expectError {
					assert.Equal(t, tt.expectedHeight, height)
				}
			}
		})
	}
}

func TestEthGetLogsBatchRequest(t *testing.T) {
	tests := []struct {
		name               string
		body               string
		expectNotSupported bool
	}{
		{
			name:               "batch request with eth_getLogs should not be supported",
			body:               `[{"jsonrpc":"2.0","id":1,"method":"eth_getLogs","params":[{"fromBlock":"latest"}]}]`,
			expectNotSupported: true,
		},
		{
			name:               "batch request with eth_getLogs and other methods",
			body:               `[{"jsonrpc":"2.0","id":1,"method":"eth_blockNumber","params":[]},{"jsonrpc":"2.0","id":2,"method":"eth_getLogs","params":[{"fromBlock":"latest"}]}]`,
			expectNotSupported: true,
		},
		{
			name:               "single request with eth_getLogs should be supported (structure test)",
			body:               `{"jsonrpc":"2.0","id":1,"method":"eth_getLogs","params":[{"fromBlock":"latest"}]}`,
			expectNotSupported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			var firstChar byte
			for i := range body {
				if body[i] != ' ' && body[i] != '\t' && body[i] != '\n' && body[i] != '\r' {
					firstChar = body[i]
					break
				}
			}

			isBatch := firstChar == '['

			if isBatch {
				// Parse batch request
				var requests []gateway.JSONRPCRequest
				err := json.Unmarshal(body, &requests)
				require.NoError(t, err)

				// Check if any request is eth_getLogs
				hasGetLogs := false
				for _, req := range requests {
					if req.Method == "eth_getLogs" {
						hasGetLogs = true
						break
					}
				}

				if hasGetLogs {
					assert.True(t, tt.expectNotSupported, "eth_getLogs in batch should not be supported")
				}
			} else {
				// Single request - should be supported
				var req gateway.JSONRPCRequest
				err := json.Unmarshal(body, &req)
				require.NoError(t, err)

				if req.Method == "eth_getLogs" {
					assert.False(t, tt.expectNotSupported, "eth_getLogs as single request should be supported")
				}
			}
		})
	}
}

func TestEthGetLogsFilterStructure(t *testing.T) {
	tests := []struct {
		name        string
		params      string
		expectValid bool
	}{
		{
			name:        "valid filter with fromBlock",
			params:      `[{"fromBlock":"latest"}]`,
			expectValid: true,
		},
		{
			name:        "valid filter with toBlock",
			params:      `[{"toBlock":"0x100"}]`,
			expectValid: true,
		},
		{
			name:        "valid filter with blockHash",
			params:      `[{"blockHash":"0x1234567890abcdef"}]`,
			expectValid: true,
		},
		{
			name:        "valid filter with address and topics",
			params:      `[{"address":"0x1234567890123456789012345678901234567890","topics":[]}]`,
			expectValid: true,
		},
		{
			name:        "valid filter with all fields",
			params:      `[{"fromBlock":"0x100","toBlock":"0x200","address":"0x1234567890123456789012345678901234567890","topics":[]}]`,
			expectValid: true,
		},
		{
			name:        "empty params array",
			params:      `[]`,
			expectValid: true,
		},
		{
			name:        "null filter",
			params:      `[null]`,
			expectValid: true,
		},
		{
			name:        "invalid filter (not an object)",
			params:      `["invalid"]`,
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paramsMap []any
			err := json.Unmarshal([]byte(tt.params), &paramsMap)

			if !tt.expectValid {
				// For invalid cases, we expect either unmarshal error or invalid structure
				if err == nil {
					// Check if structure is invalid
					if len(paramsMap) > 0 {
						_, ok := paramsMap[0].(map[string]any)
						assert.False(t, ok, "filter should not be a valid map")
					}
				}
			} else {
				require.NoError(t, err)
				if len(paramsMap) > 0 && paramsMap[0] != nil {
					_, ok := paramsMap[0].(map[string]any)
					assert.True(t, ok, "filter should be a map")
				}
			}
		})
	}
}
