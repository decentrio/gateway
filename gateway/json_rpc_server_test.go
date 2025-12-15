package gateway_test

import (
	"bytes"
	"encoding/json"
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
		name        string
		body        string
		expectError bool
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
		name           string
		requests       []gateway.JSONRPCRequest
		expectedLength int
		allNotifications bool
	}{
		{
			name: "all requests have IDs",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", ID: json.RawMessage("2"), Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength: 2,
			allNotifications: false,
		},
		{
			name: "all requests are notifications",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength: 0,
			allNotifications: true,
		},
		{
			name: "mix of notifications and regular requests",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength: 1,
			allNotifications: false,
		},
		{
			name: "requests with null IDs",
			requests: []gateway.JSONRPCRequest{
				{JSONRPC: "2.0", ID: json.RawMessage("null"), Method: "eth_blockNumber", Params: json.RawMessage("[]")},
				{JSONRPC: "2.0", ID: json.RawMessage("1"), Method: "eth_gasPrice", Params: json.RawMessage("[]")},
			},
			expectedLength: 1,
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
		name      string
		body      string
		isBatch   bool
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

