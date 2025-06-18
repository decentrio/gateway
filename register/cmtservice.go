package register

import (
	"context"
	"crypto/tls"
	"fmt"

	// "fmt"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	"github.com/decentrio/gateway/config"
	"github.com/gogo/status"
)

type CustomTMService struct {
	tmservice.UnimplementedServiceServer
}

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

// todo: manually add other query types if needed