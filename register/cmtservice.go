package register

import (
	"context"
	"crypto/tls"
	// "fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	// "google.golang.org/grpc/credentials/insecure"
	// "google.golang.org/grpc/metadata"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/decentrio/gateway/config"
	"github.com/gogo/status"
)

type CustomTMService struct {
	tmservice.UnimplementedServiceServer
}

func (s *CustomTMService) GetBlockByHeight(ctx context.Context, req *tmservice.GetBlockByHeightRequest) (*tmservice.GetBlockByHeightResponse, error) {
	// Tạo metadata từ height
	// md := metadata.New(map[string]string{
	// 	"x-cosmos-block-height": fmt.Sprintf("%d", req.Height),
	// })

	// Tạo context mới có metadata
	// ctx = metadata.NewOutgoingContext(ctx, md)

	// Lấy node từ config theo height
	node := config.GetNodebyHeight(uint64(req.Height))
	if node == nil {
		return nil, status.Errorf(codes.InvalidArgument, "No node found for height %d", req.Height)
	}

	var err error
	var conn *grpc.ClientConn
	if strings.HasSuffix(node.GRPC, ":443") {
		tlsConfig := &tls.Config{InsecureSkipVerify: true}
		conn, err = grpc.DialContext(ctx, node.GRPC, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	} else {
		conn, err = grpc.DialContext(ctx, node.GRPC, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// Dial node gRPC
	// creds := credentials.NewClientTLSFromCert(nil, "")
	// conn, err := grpc.Dial(node.GRPC, grpc.WithTransportCredentials(creds))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Tạo client gốc
	client := tmservice.NewServiceClient(conn)

	// Gửi lại request tới backend
	return client.GetBlockByHeight(ctx, req)

}
