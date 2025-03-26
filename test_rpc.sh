#!/bin/bash

GATEWAY_URL="http://localhost:5001" 

declare -a requests=(
  '{"jsonrpc": "2.0", "id": 1, "method": "block", "params": {}}'
  '{"jsonrpc": "2.0", "id": 2, "method": "status", "params": {}}'
  '{"jsonrpc": "2.0", "id": 3, "method": "block", "params": {"height": "100"}}'
  '{"jsonrpc": "2.0", "id": 4, "method": "block", "params": {"height": ["31111"]}}'
  '{"jsonrpc": "2.0", "id": 5, "method": "block", "params": {"height": {"value": "61111"}}}'
  '{"jsonrpc": "2.0", "id": 6, "method": "blockchain", "params": {"maxHeight": "150"}}'
  '{"jsonrpc": "2.0", "id": 7, "method": "block_by_hash", "params": {"hash": "0xabcdef123456"}}'
  '{"jsonrpc": "2.0", "id": 8, "method": "tx_search", "params": {"query": "tx.height=100"}}'
  '{"jsonrpc": "2.0", "id": 9, "method": "invalid_method", "params": {}}'
)

ENDPOINTS=(
    "/abci_info"
    "/broadcast_tx_async"
    "/genesis"
    "/health"
    "/status"
    "/block?height=100"
    "/validators?height=100"
    "/blockchain?maxHeight=31000"
    "/tx_search?query=\"tx.height>100\""
)

echo "🚀 Starting RPC Gateway Tests at $GATEWAY_URL"
echo "============================================"

# 🛠 Test JSON-RPC requests
echo "📡 Testing JSON-RPC API"
echo "--------------------------------------"
for req in "${requests[@]}"
do
  echo "🔹 Sending JSON-RPC request: $req"
  response=$(curl -s -X POST "$GATEWAY_URL" -H "Content-Type: application/json" -d "$req")
  echo "Response: $response"
  echo "--------------------------------------"
done

# 🛠 Test REST API endpoints
echo "🌍 Testing REST API Endpoints"
echo "--------------------------------------"
for endpoint in "${ENDPOINTS[@]}"; do
    echo "🔹 Testing: $endpoint"
    response_code=$(curl -s -o /dev/null -w "%{http_code}" "$GATEWAY_URL$endpoint")

    if [ "$response_code" -eq 200 ]; then
        echo "✅ Success: $endpoint"
    else
        echo "❌ Failed: $endpoint (HTTP $response_code)"
    fi

    echo "--------------------------------------"
done

echo "🎯 All tests completed!"
