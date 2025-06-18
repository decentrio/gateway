package register

import (
	"google.golang.org/grpc"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
)

func Register(grpcServer *grpc.Server) {
	tmservice.RegisterServiceServer(grpcServer, &CustomTMService{})
	// add service
}
