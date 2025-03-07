package gateway

import (
	"fmt"
	"os"
	"net/http"
	"strconv"

	"github.com/decentrio/gateway/config"
	"google.golang.org/grpc"

)

type Server interface {
	Start()
	Shutdown()
}

type Gateway struct {
	RPCServer Server 
	GRPCServer Server
	APIServer Server
	JSONRPCServer Server
	JSONRPCWSServer Server
}



