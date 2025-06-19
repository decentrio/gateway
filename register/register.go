package register

import (
	"google.golang.org/grpc"

	tmservice "github.com/cosmos/cosmos-sdk/client/grpc/tmservice"
	txsservice "github.com/cosmos/cosmos-sdk/types/tx"
)

func Register(grpcServer *grpc.Server) {
	tmservice.RegisterServiceServer(grpcServer, &CustomTMService{})
	txsservice.RegisterServiceServer(grpcServer, &CustomTxsService{})
	// add service
}
