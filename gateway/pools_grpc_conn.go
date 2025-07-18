package gateway

import (
	"context"
	"crypto/tls"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var connPool = make(map[string]*grpc.ClientConn)
var poolMu sync.RWMutex

func getGRPCConn(ctx context.Context, addr string) (*grpc.ClientConn, error) {
	poolMu.RLock()
	conn, ok := connPool[addr]
	poolMu.RUnlock()

	if ok {
		return conn, nil
	}

	poolMu.Lock()
	defer poolMu.Unlock()
	// Double check to avoid race
	if conn, ok := connPool[addr]; ok {
		return conn, nil
	}
	var opts grpc.DialOption
	if strings.HasSuffix(addr, ":443") {
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		opts = grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))
	} else {
		opts = grpc.WithTransportCredentials(insecure.NewCredentials())
	}

	newConn, err := grpc.DialContext(ctx, addr, opts)

	if err != nil {
		return nil, err
	}
	connPool[addr] = newConn
	return newConn, nil
}

func closeAllGRPCConnections() {
	poolMu.Lock()
	defer poolMu.Unlock()
	for addr, conn := range connPool {
		conn.Close()
		delete(connPool, addr)
	}
}
