# Gateway

A gateway server for multiple cosmos nodes. Redirect requests to the corresponding nodes by height.

## Start the Gateway

```bash
gateway start --config config.yaml
```

## Config file syntax:

```yaml
# config.yaml

#  block range will define type of nodes:
#  - [1000]: Subnode with a range of latest 1000 blocks. This should be placed at the top for heighest priority over other nodes.
#  - [1, 1000]: Subnode with specified block range
#  - [1, 0]: Subnode with specified block range to the latest block (for querying without specifying block height)

#  List of sub nodes, with endpoints and port ranges.
upstream:
  - rpc: "http://node1:26657"
    api: "http://node1:1317"
    grpc: "node1:9090"
    jsonrpc: "http://node1:8545"
    jsonrpc_ws: "ws://node1:8546"
    blocks: [1000, 2000]
  - ...

# Gateway's custom port
# If a port is set to 0, the service of that port won't start.
port:
    rpc: 26657
    api: 0  # Disable API service
    grpc: 9090
    jsonrpc: 8545
    jsonrpc_ws: 8546
```

## Endpoint Structure

- API, RPC: [Postman Collection](https://www.postman.com/flight-astronomer-81853429/osmosis)
- JSON RPC: [Ethereum JSON-RPC Documentation](https://documenter.getpostman.com/view/4117254/ethereum-json-rpc/RVu7CT5J)

## Testing

### RPC

- **GET Request**
  ```bash
  curl "localhost:5001/block?"
  ```
- **POST Request**
  ```bash
  curl -X POST "https://gw.rpc.decentrio.ventures" -d '{
      "jsonrpc":"2.0",
      "id":0,
      "method":"tx",
      "params": {
          "hash":"ZN/cD0uQlq38ZEst8IfnuSJchgFxnEwrsul5rYMIFxM=",
          "prove":true
      }
  }'
  ```
- **CLI Example**
  ```bash
  binaryd --node http://localhost:5001 q tx 64DFDC0F4B9096ADFC644B2DF087E7B9225C8601719C4C2BB2E979AD83081713
  ```

### API

> **Note:** Swagger does not work.

### gRPC

- **Using GrpcUI**
  ```bash
  grpcui -plaintext localhost:5002
  ```
- **List Available Services**
  ```bash
  grpcurl -plaintext localhost:5002 list
  ```
- **List Available Methods for a Specific Service**
  ```bash
  grpcurl -plaintext localhost:5002 list <service_name>
  ```
- **Call a gRPC Method**
  ```bash
  grpcurl -plaintext -d '{"param1": "value1", "param2": "value2"}' localhost:5002 <service_name>/<method_name>
  ```
- **Check Server Reflection**
  ```bash
  grpcurl -plaintext localhost:5002 describe
  ```
- **Get Details of a Specific Method**
  ```bash
  grpcurl -plaintext localhost:5002 describe <service_name>/<method_name>
  ```
- **Examples:**
  - **With Headers**
    ```bash
    grpcurl -d '{"height": "123"}' \
      -H "x-cosmos-block-height: 123" \
      -plaintext \
      localhost:5002 cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight
    ```
  - **Without Headers**
    ```bash
    grpcurl -d '{"height": "123"}' \
      -plaintext \
      localhost:5002 cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight
    ```
  - **Get Transaction Info**
    ```bash
    grpcurl -plaintext -d '{"hash": "64DFDC0F4B9096ADFC644B2DF087E7B9225C8601719C4C2BB2E979AD83081713"}' \
        localhost:5002 cosmos.tx.v1beta1.Service/GetTx
    ```

### JSON RPC

```bash
curl -X POST "https://gw-jr.rpc.decentrio.ventures" -d '{
        "jsonrpc":"2.0",
        "method":"eth_getBlockByHash",
        "params":[
                "0x68f04262ea363216fae99a7498502075c6aacc42bdc4db7c29e7f64c2fab0fda",
                true
        ],
        "id":1
}' -H "Content-Type: application/json"
```

### JSON RPC WebSocket

- **Send a JSON-RPC request via WebSocket using websocat:**
  ```bash
  echo -n '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' | websocat ws://localhost:5006/websocket
  ```
- **Interactive Mode:**
  ```bash
  websocat ws://localhost:5006/websocket
  ```
  Then send requests manually, for example:
  ```bash
  {"jsonrpc":"2.0","method":"eth_getBlockByHash","params":["0xedf27a6af5a10e72102b0ba73940fd3b9fb21900b822178405bbd2a969e408fb", true],"id":1}
  ```
  ```bash
  {"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x43", true],"id":1}
  ```

