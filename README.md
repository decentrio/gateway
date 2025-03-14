## Testing
```bash
curl -X POST "http://localhost:5001" -d '{
    "jsonrpc":"2.0",
    "id":0,
    "method":"tx",
    "params": {
        "hash":"ZN/cD0uQlq38ZEst8IfnuSJchgFxnEwrsul5rYMIFxM=",
        "prove":true
    }
}'
```

64DFDC0F4B9096ADFC644B2DF087E7B9225C8601719C4C2BB2E979AD83081713