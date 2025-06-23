package register

import (
	"context"
	"crypto/tls"
	"fmt"

	// "fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/tmservice"

	"github.com/decentrio/gateway/config"
)

type CustomTMService struct {
	tmservice.UnimplementedServiceServer
}

// grpcurl -plaintext -d '{"height":"12"}' localhost:5002 cosmos.base.tendermint.v1beta1.Service.GetBlockByHeight
func (s *CustomTMService) GetBlockByHeight(ctx context.Context, req *tmservice.GetBlockByHeightRequest) (*tmservice.GetBlockByHeightResponse, error) {
	client, conn := getClientTm(ctx, req.Height)
	defer conn.Close()
	return client.GetBlockByHeight(ctx, req)
}

// grpcurl -plaintext -d '{"height":"12"}' localhost:5002 cosmos.base.tendermint.v1beta1.Service.GetValidatorSetByHeight
func (s *CustomTMService) GetValidatorSetByHeight(ctx context.Context, req *tmservice.GetValidatorSetByHeightRequest) (*tmservice.GetValidatorSetByHeightResponse, error) {
	client, conn := getClientTm(ctx, req.Height)
	defer conn.Close()
	return client.GetValidatorSetByHeight(ctx, req)

}

//	grpcurl -plaintext -d '{
//		"path": "/store/bank/key",
//		"data": "0a2d636f736d6f73316c71733763746e393578386d3930347a6766786a646b7777766638746b6c6b707936656b"
//	  }' localhost:5002 cosmos.base.tendermint.v1beta1.Service.ABCIQuery
func (s *CustomTMService) ABCIQuery(ctx context.Context, req *tmservice.ABCIQueryRequest) (*tmservice.ABCIQueryResponse, error) {
	client, conn := getClientTm(ctx, req.Height)
	defer conn.Close()
	return client.ABCIQuery(ctx, req)
}

func (*CustomTMService) GetLatestBlock(ctx context.Context, req *tmservice.GetLatestBlockRequest) (*tmservice.GetLatestBlockResponse, error) {
	client, conn := getClientTm(ctx, 0)
	defer conn.Close()
	return client.GetLatestBlock(ctx, req)
}

func (*CustomTMService) GetSyncing(ctx context.Context, req *tmservice.GetSyncingRequest) (*tmservice.GetSyncingResponse, error) {
	client, conn := getClientTm(ctx, 0)
	defer conn.Close()
	return client.GetSyncing(ctx, req)
}

func (*CustomTMService) GetNodeInfo(ctx context.Context, req *tmservice.GetNodeInfoRequest) (*tmservice.GetNodeInfoResponse, error) {
	client, conn := getClientTm(ctx, 0)
	defer conn.Close()
	return client.GetNodeInfo(ctx, req)
}

func getClientTm(ctx context.Context, height int64) (tmservice.ServiceClient, *grpc.ClientConn) {
	node := config.GetNodebyHeight(uint64(height))
	if node == nil {
		return nil, nil
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
		return nil, nil
	}

	return tmservice.NewServiceClient(conn), conn
}
