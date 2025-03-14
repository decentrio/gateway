package gateway_test

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/decentrio/gateway/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

func TestDirector_SelectsCorrectNode(t *testing.T) {
	mockConfig := new(MockConfig)
	cfg := &config.Config{Upstream: []config.Node{
		{GRPC: "node1:9090", Blocks: []uint64{1000, 2000}},
		{GRPC: "node2:9090", Blocks: []uint64{2001, 3000}},
	}}

	mockConfig.On("GetNodebyHeight", uint64(1500)).Return(&cfg.Upstream[0])

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-cosmos-block-height", "1500"))
	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		height, _ := strconv.ParseUint(md["x-cosmos-block-height"][0], 10, 64)
		node := mockConfig.GetNodebyHeight(height)
		if node == nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "No matching backend found")
		}
		conn, err := grpc.Dial(node.GRPC, grpc.WithInsecure())
		return outCtx, conn, err
	}

	_, conn, err := director(ctx, "/test.Service/Method")
	assert.NoError(t, err)
	assert.NotNil(t, conn)
	mockConfig.AssertCalled(t, "GetNodebyHeight", uint64(1500))
}

func TestDirector_NoMatchingNode(t *testing.T) {
	mockConfig := new(MockConfig)
	mockConfig.On("GetNodebyHeight", uint64(9999)).Return(nil)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-cosmos-block-height", "9999"))
	director := func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		height, _ := strconv.ParseUint(md["x-cosmos-block-height"][0], 10, 64)
		node := mockConfig.GetNodebyHeight(height)
		if node == nil {
			return nil, nil, status.Errorf(codes.InvalidArgument, "No matching backend found")
		}
		return outCtx, nil, errors.New("unexpected")
	}

	_, conn, err := director(ctx, "/test.Service/Method")
	assert.Error(t, err)
	assert.Nil(t, conn)
	mockConfig.AssertCalled(t, "GetNodebyHeight", uint64(9999))
}
