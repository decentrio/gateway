package register

import (
	"google.golang.org/grpc"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/cmtservice"
)

func Register(grpcServer *grpc.Server) {
	tmservice.RegisterServiceServer(grpcServer, &CustomTMService{})
	// add service
}
