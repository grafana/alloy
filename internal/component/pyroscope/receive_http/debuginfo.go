package receive_http

import (
	"context"
	"errors"
	"io"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	debuginfopb "buf.build/gen/go/parca-dev/parca/protocolbuffers/go/parca/debuginfo/v1alpha1"
	"github.com/gorilla/mux"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errNoDebugInfoClient = status.Error(codes.Unavailable, "no debug info client available")

//nolint:unused
func (c *Component) mountDebugInfo(router *mux.Router) {
	c.grpcServer = NewGrpcServer(c.server.Config())
	debuginfogrpc.RegisterDebuginfoServiceServer(c.grpcServer, c)
	const (
		DebuginfoService_Upload_FullMethodName               = "/parca.debuginfo.v1alpha1.DebuginfoService/Upload"
		DebuginfoService_ShouldInitiateUpload_FullMethodName = "/parca.debuginfo.v1alpha1.DebuginfoService/ShouldInitiateUpload"
		DebuginfoService_InitiateUpload_FullMethodName       = "/parca.debuginfo.v1alpha1.DebuginfoService/InitiateUpload"
		DebuginfoService_MarkUploadFinished_FullMethodName   = "/parca.debuginfo.v1alpha1.DebuginfoService/MarkUploadFinished"
	)
	router.PathPrefix(DebuginfoService_Upload_FullMethodName).Handler(c.grpcServer)
	router.PathPrefix(DebuginfoService_ShouldInitiateUpload_FullMethodName).Handler(c.grpcServer)
	router.PathPrefix(DebuginfoService_InitiateUpload_FullMethodName).Handler(c.grpcServer)
	router.PathPrefix(DebuginfoService_MarkUploadFinished_FullMethodName).Handler(c.grpcServer)
}

func (c *Component) getDebugInfoClient() debuginfogrpc.DebuginfoServiceClient {
	c.mut.Lock()
	defer c.mut.Unlock()
	for _, appendable := range c.appendables {
		if client := appendable.Client(); client != nil {
			return client
		}
	}
	return nil
}

func (c *Component) Upload(stream grpc.ClientStreamingServer[debuginfopb.UploadRequest, debuginfopb.UploadResponse]) error {
	client := c.getDebugInfoClient()
	if client == nil {
		return errNoDebugInfoClient
	}

	upstreamContext, upstreamCancel := context.WithCancel(stream.Context())
	defer upstreamCancel()
	upstreamStream, err := client.Upload(upstreamContext)
	if err != nil {
		return err
	}

	for {
		req, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			resp, err := upstreamStream.CloseAndRecv()
			if err != nil {
				return err
			}
			return stream.SendAndClose(resp)
		}
		if err != nil {
			return err
		}

		if err := upstreamStream.Send(req); err != nil {
			return err
		}
	}
}

func (c *Component) ShouldInitiateUpload(ctx context.Context, request *debuginfopb.ShouldInitiateUploadRequest) (*debuginfopb.ShouldInitiateUploadResponse, error) {
	client := c.getDebugInfoClient()
	if client == nil {
		return nil, errNoDebugInfoClient
	}
	return client.ShouldInitiateUpload(ctx, request)
}

func (c *Component) InitiateUpload(ctx context.Context, request *debuginfopb.InitiateUploadRequest) (*debuginfopb.InitiateUploadResponse, error) {
	client := c.getDebugInfoClient()
	if client == nil {
		return nil, errNoDebugInfoClient
	}
	return client.InitiateUpload(ctx, request)
}

func (c *Component) MarkUploadFinished(ctx context.Context, request *debuginfopb.MarkUploadFinishedRequest) (*debuginfopb.MarkUploadFinishedResponse, error) {
	client := c.getDebugInfoClient()
	if client == nil {
		return nil, errNoDebugInfoClient
	}
	return client.MarkUploadFinished(ctx, request)
}
