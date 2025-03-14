## Testing
### RPC
 - `GET` method:
    ```bash
    curl "localhost:5001/block?"
    ```
 - `POST` method:
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
- cli:
    ```bash
    binaryd --node http://localhost:5001 q tx 64DFDC0F4B9096ADFC644B2DF087E7B9225C8601719C4C2BB2E979AD83081713
    ```
### API
 > Note: swagger does not work


### gRPC
##### List available services:
```bash
grpcurl -plaintext localhost:5002 list
```

##### List available methods for a specific service:
```bash
grpcurl -plaintext localhost:5002 list <service_name>
```

##### Call a gRPC method (example):
```bash
grpcurl -plaintext -d '{"param1": "value1", "param2": "value2"}' localhost:5002 <service_name>/<method_name>
```

##### Check server reflection:
```bash
grpcurl -plaintext localhost:5002 describe
```

##### Get details of a specific method:
```bash
grpcurl -plaintext localhost:5002 describe <service_name>/<method_name>
```

##### Example:
- Header:
```bash
grpcurl -v -d '{"height": "123"}' \
  -H "x-cosmos-block-height: 123" \
  -plaintext \
  localhost:5002 cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight
```
- No header:
```bash
grpcurl -v -d '{"height": "123"}' \
  -plaintext \
  localhost:5002 cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight
```
- Get tx info:
```bash
grpcurl -plaintext -d '{"hash": "64DFDC0F4B9096ADFC644B2DF087E7B9225C8601719C4C2BB2E979AD83081713"}' \
    localhost:5002 cosmos.tx.v1beta1.Service/GetTx
```
