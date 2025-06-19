package register

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/gogo/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	txsservice "github.com/cosmos/cosmos-sdk/types/tx"

	"github.com/decentrio/gateway/config"
)

type CustomTxsService struct {
	txsservice.UnimplementedServiceServer
}

func (s *CustomTxsService) GetBlockWithTxs(ctx context.Context, req *txsservice.GetBlockWithTxsRequest) (*txsservice.GetBlockWithTxsResponse, error) {
	node := config.GetNodebyHeight(uint64(req.Height))
	if node == nil {
		return nil, status.Errorf(codes.InvalidArgument, "No node found for height %d", req.Height)
	}

	fmt.Printf("Forwarding GetBlockByHeight request to node: %s\n", node.GRPC)

	var err error
	var conn *grpc.ClientConn
	if strings.HasSuffix(node.GRPC, ":443") {
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		conn, err = grpc.DialContext(ctx, node.GRPC, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		conn, err = grpc.DialContext(ctx, node.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := txsservice.NewServiceClient(conn)
	return client.GetBlockWithTxs(ctx, req)

}
