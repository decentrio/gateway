package gateway

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"

	"github.com/decentrio/gateway/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type MockConfig struct {
	mock.Mock
}

func (m *MockConfig) GetNodebyHeight(height uint64) *config.Node {
	args := m.Called(height)
	if node, ok := args.Get(0).(*config.Node); ok {
		return node
	}
	return nil
}

func (m *MockConfig) GetNodesByType(nodeType string) []*config.Node {
	args := m.Called(nodeType)
	if nodes, ok := args.Get(0).([]*config.Node); ok {
		return nodes
	}
	return nil
}

func startMockGRPCServer(t *testing.T, address string) {
	lis, err := net.Listen("tcp", address)
	assert.NoError(t, err)

	grpcServer := grpc.NewServer()
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Errorf("Failed to start mock gRPC server: %v", err)
		}
	}()
	t.Cleanup(func() {
		grpcServer.GracefulStop()
	})
}

func TestGrpcForwarding(t *testing.T) {
	mockConfig := new(MockConfig)

	mockConfig.On("GetNodebyHeight", uint64(123)).Return(&config.Node{GRPC: "localhost:9091"})
	mockConfig.On("GetNodesByType", "grpc").Return([]*config.Node{{GRPC: "localhost:9092"}})

	startMockGRPCServer(t, "localhost:9091")

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-cosmos-block-height", "123"))

	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, nil, status.Errorf(codes.Unimplemented, "Unknown method")
		}
		fmt.Printf("Received metadata: %+v\n", md)

		heightVals, exists := md["x-cosmos-block-height"]
		if !exists || len(heightVals) == 0 {
			return nil, nil, status.Errorf(codes.InvalidArgument, "Missing x-cosmos-block-height")
		}

		height, err := strconv.ParseUint(heightVals[0], 10, 64)
		if err != nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "Invalid x-cosmos-block-height")
		}

		node := mockConfig.GetNodebyHeight(height)
		if node == nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "No matching backend found")
		}
		fmt.Printf("Forwarding request to: %s\n", node.GRPC)

		conn, err := grpc.Dial(node.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return ctx, conn, err
	}

	_, conn, err := director(ctx, "cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight")
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, "localhost:9091", conn.Target())

	ctxNoHeight := metadata.NewIncomingContext(context.Background(), metadata.Pairs())
	_, _, errNoHeight := director(ctxNoHeight, "cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight")
	assert.Error(t, errNoHeight)
	assert.Equal(t, codes.InvalidArgument, status.Code(errNoHeight))

	mockConfig.On("GetNodebyHeight", uint64(999)).Return(nil)
	ctxInvalid := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-cosmos-block-height", "999"))
	_, _, errInvalid := director(ctxInvalid, "cosmos.base.tendermint.v1beta1.Service/GetBlockByHeight")
	assert.Error(t, errInvalid)
	assert.Equal(t, codes.InvalidArgument, status.Code(errInvalid))
}
