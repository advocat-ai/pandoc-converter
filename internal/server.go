package internal

import (
	"fmt"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net"
)

func NewServerAndListener(protocol string, address string, logger *zap.Logger) (*grpc.Server, net.Listener, error) {
	log := logger.With(zap.String("bindAddress", fmt.Sprintf("%v://%v", protocol, address)))
	log.Debug("creating listener")
	listener, err := net.Listen(protocol, address)
	if err != nil {
		log.Error("failed to create listener", zap.Error(err))
		return nil, nil, err
	}
	log.Debug("listener created")

	server := grpc.NewServer()

	return server, listener, nil
}