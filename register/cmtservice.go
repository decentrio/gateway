package register

import (
	"context"
	"crypto/tls"
	"fmt"

	// "fmt"
	"strings"

	"github.com/gogo/status"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

	client := tmservice.NewServiceClient(conn)
	return client.GetBlockByHeight(ctx, req)

}

// grpcurl -plaintext -d '{"height":"12"}' localhost:5002 cosmos.base.tendermint.v1beta1.Service.GetValidatorSetByHeight
func (s *CustomTMService) GetValidatorSetByHeight(ctx context.Context, req *tmservice.GetValidatorSetByHeightRequest) (*tmservice.GetValidatorSetByHeightResponse, error) {
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

	client := tmservice.NewServiceClient(conn)
	return client.GetValidatorSetByHeight(ctx, req)

}

//	grpcurl -plaintext -d '{
//		"path": "/store/bank/key",
//		"data": "0a2d636f736d6f73316c71733763746e393578386d3930347a6766786a646b7777766638746b6c6b707936656b"
//	  }' localhost:5002 cosmos.base.tendermint.v1beta1.Service.ABCIQuery
func (s *CustomTMService) ABCIQuery(ctx context.Context, req *tmservice.ABCIQueryRequest) (*tmservice.ABCIQueryResponse, error) {
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

	client := tmservice.NewServiceClient(conn)
	return client.ABCIQuery(ctx, req)

}

// todo: manually add other query types if needed
