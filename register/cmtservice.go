package register

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
	"github.com/decentrio/gateway/config"
	"github.com/gogo/status"
)

type CustomTMService struct {
	tmservice.UnimplementedServiceServer
}

func (s *CustomTMService) GetBlockByHeight(ctx context.Context, req *tmservice.GetBlockByHeightRequest) (*tmservice.GetBlockByHeightResponse, error) {
	// Tạo metadata từ height
	md := metadata.New(map[string]string{
		"x-cosmos-block-height": fmt.Sprintf("%d", req.Height),
	})

	// Tạo context mới có metadata
	ctx = metadata.NewOutgoingContext(ctx, md)

	// Lấy node từ config theo height
	node := config.GetNodebyHeight(uint64(req.Height))
	if node == nil {
		return nil, status.Errorf(codes.InvalidArgument, "No node found for height %d", req.Height)
	}

	// Dial node gRPC
	conn, err := grpc.Dial(node.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Tạo client gốc
	client := tmservice.NewServiceClient(conn)

	// Gửi lại request tới backend
	return client.GetBlockByHeight(ctx, req)

}
