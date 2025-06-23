package register

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	"google.golang.org/grpc"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	txsservice "github.com/cosmos/cosmos-sdk/types/tx"

	"github.com/decentrio/gateway/config"
)

type CustomTxsService struct {
	txsservice.UnimplementedServiceServer
}

func (s *CustomTxsService) BroadcastTx(ctx context.Context, req *txsservice.BroadcastTxRequest) (*txsservice.BroadcastTxResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.BroadcastTx(ctx, req)
}
func (s *CustomTxsService) GetBlockWithTxs(ctx context.Context, req *txsservice.GetBlockWithTxsRequest) (*txsservice.GetBlockWithTxsResponse, error) {
	client, conn := getClientTxs(ctx, req.Height)
	defer conn.Close()
	return client.GetBlockWithTxs(ctx, req)
}
func (s *CustomTxsService) GetTx(ctx context.Context, req *txsservice.GetTxRequest) (*txsservice.GetTxResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.GetTx(ctx, req)
}

func (s *CustomTxsService) GetTxsEvent(ctx context.Context, req *txsservice.GetTxsEventRequest) (*txsservice.GetTxsEventResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.GetTxsEvent(ctx, req)
}
func (s *CustomTxsService) Simulate(ctx context.Context, req *txsservice.SimulateRequest) (*txsservice.SimulateResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.Simulate(ctx, req)
}
func (s *CustomTxsService) TxDecode(ctx context.Context, req *txsservice.TxDecodeRequest) (*txsservice.TxDecodeResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.TxDecode(ctx, req)
}
func (s *CustomTxsService) TxDecodeAmino(ctx context.Context, req *txsservice.TxDecodeAminoRequest) (*txsservice.TxDecodeAminoResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.TxDecodeAmino(ctx, req)
}
func (s *CustomTxsService) TxEncode(ctx context.Context, req *txsservice.TxEncodeRequest) (*txsservice.TxEncodeResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.TxEncode(ctx, req)
}
func (s *CustomTxsService) TxEncodeAmino(ctx context.Context, req *txsservice.TxEncodeAminoRequest) (*txsservice.TxEncodeAminoResponse, error) {
	client, conn := getClientTxs(ctx, 0)
	defer conn.Close()
	return client.TxEncodeAmino(ctx, req)
}

func getClientTxs(ctx context.Context, height int64) (txsservice.ServiceClient, *grpc.ClientConn) {
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

	return txsservice.NewServiceClient(conn), conn
}
