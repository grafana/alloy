package receive_http

import (
	"context"

	debuginfopb "buf.build/gen/go/parca-dev/parca/protocolbuffers/go/parca/debuginfo/v1alpha1"
	"github.com/grafana/dskit/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

//todo grep for grpc server configurations in alloy

func NewGrpcServer(cfg server.Config) *grpc.Server {
	grpcKeepAliveOptions := keepalive.ServerParameters{
		MaxConnectionIdle:     cfg.GRPCServerMaxConnectionIdle,
		MaxConnectionAge:      cfg.GRPCServerMaxConnectionAge,
		MaxConnectionAgeGrace: cfg.GRPCServerMaxConnectionAgeGrace,
		Time:                  cfg.GRPCServerTime,
		Timeout:               cfg.GRPCServerTimeout,
	}

	grpcKeepAliveEnforcementPolicy := keepalive.EnforcementPolicy{
		MinTime:             cfg.GRPCServerMinTimeBetweenPings,
		PermitWithoutStream: cfg.GRPCServerPingWithoutStreamAllowed,
	}

	grpcOptions := []grpc.ServerOption{
		grpc.KeepaliveParams(grpcKeepAliveOptions),
		grpc.KeepaliveEnforcementPolicy(grpcKeepAliveEnforcementPolicy),
		grpc.MaxRecvMsgSize(cfg.GRPCServerMaxRecvMsgSize),
		grpc.MaxSendMsgSize(cfg.GRPCServerMaxSendMsgSize),
		grpc.MaxConcurrentStreams(uint32(cfg.GRPCServerMaxConcurrentStreams)),
		grpc.NumStreamWorkers(uint32(cfg.GRPCServerNumWorkers)),
	}

	grpcOptions = append(grpcOptions, cfg.GRPCOptions...)

	return grpc.NewServer(grpcOptions...)
}

func (c *Component) Upload(g grpc.ClientStreamingServer[debuginfopb.UploadRequest, debuginfopb.UploadResponse]) error {
	c.appendables[0].Appender()
	//TODO implement me
	panic("implement me")
}

func (c *Component) ShouldInitiateUpload(ctx context.Context, request *debuginfopb.ShouldInitiateUploadRequest) (*debuginfopb.ShouldInitiateUploadResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Component) InitiateUpload(ctx context.Context, request *debuginfopb.InitiateUploadRequest) (*debuginfopb.InitiateUploadResponse, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Component) MarkUploadFinished(ctx context.Context, request *debuginfopb.MarkUploadFinishedRequest) (*debuginfopb.MarkUploadFinishedResponse, error) {
	//TODO implement me
	panic("implement me")
}
