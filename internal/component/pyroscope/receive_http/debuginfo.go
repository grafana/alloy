package receive_http

import (
	"context"
	"fmt"
	"io"
	"sync"

	"connectrpc.com/connect"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
)

func (c *Component) getDebugInfoConnectClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	c.mut.Lock()
	defer c.mut.Unlock()
	var clients []debuginfov1alpha1connect.DebuginfoServiceClient
	for _, appendable := range c.appendables {
		clients = append(clients, appendable.ConnectClients()...)
	}
	return clients
}

// Upload implements debuginfov1alpha1connect.DebuginfoServiceHandler.
// It fans out the upload to all downstream Connect clients.
func (c *Component) Upload(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
	clients := c.getDebugInfoConnectClients()
	if len(clients) == 0 {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("no debug info clients available"))
	}

	// Step 1: Receive the init request from the caller.
	initReq, err := stream.Receive()
	if err != nil {
		return fmt.Errorf("receive init request: %w", err)
	}
	if initReq.GetInit() == nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expected init request"))
	}

	// Step 2: For each downstream client, open a bidi stream and send the init request.
	type downstreamState struct {
		stream *connect.BidiStreamForClient[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]
		should bool
		reason string
	}

	downstreams := make([]*downstreamState, 0, len(clients))
	anyShouldUpload := false

	for _, client := range clients {
		ds := &downstreamState{}
		ds.stream = client.Upload(ctx)

		// Send init request.
		if err := ds.stream.Send(initReq); err != nil {
			_ = ds.stream.CloseRequest()
			_ = ds.stream.CloseResponse()
			continue
		}

		// Receive init response.
		resp, err := ds.stream.Receive()
		if err != nil {
			_ = ds.stream.CloseRequest()
			_ = ds.stream.CloseResponse()
			continue
		}

		initResp := resp.GetInit()
		if initResp == nil {
			_ = ds.stream.CloseRequest()
			_ = ds.stream.CloseResponse()
			continue
		}

		ds.should = initResp.ShouldInitiateUpload
		ds.reason = initResp.Reason

		if ds.should {
			anyShouldUpload = true
		} else {
			// Close streams for clients that don't need upload.
			_ = ds.stream.CloseRequest()
			_ = ds.stream.CloseResponse()
		}

		downstreams = append(downstreams, ds)
	}

	// Step 3: Send the merged response back to the caller.
	mergedReason := ""
	if !anyShouldUpload {
		for _, ds := range downstreams {
			if ds.reason != "" {
				mergedReason = ds.reason
				break
			}
		}
	}

	if err := stream.Send(&debuginfov1alpha1.UploadResponse{
		Data: &debuginfov1alpha1.UploadResponse_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: anyShouldUpload,
				Reason:               mergedReason,
			},
		},
	}); err != nil {
		// Clean up all open streams.
		for _, ds := range downstreams {
			if ds.should {
				_ = ds.stream.CloseRequest()
				_ = ds.stream.CloseResponse()
			}
		}
		return fmt.Errorf("send init response: %w", err)
	}

	if !anyShouldUpload {
		return nil
	}

	// Step 4: Stream chunks from caller to all approved downstream clients.
	activeStreams := make([]*downstreamState, 0)
	for _, ds := range downstreams {
		if ds.should {
			activeStreams = append(activeStreams, ds)
		}
	}

	for {
		req, err := stream.Receive()
		if err == io.EOF || err != nil {
			break
		}

		chunk := req.GetChunk()
		if chunk == nil {
			continue
		}

		// Fan-out chunk to all active downstream clients concurrently.
		var wg sync.WaitGroup
		for _, ds := range activeStreams {
			wg.Add(1)
			go func(ds *downstreamState) {
				defer wg.Done()
				_ = ds.stream.Send(req)
			}(ds)
		}
		wg.Wait()
	}

	// Step 5: Close all active downstream streams.
	for _, ds := range activeStreams {
		_ = ds.stream.CloseRequest()
		_ = ds.stream.CloseResponse()
	}

	return nil
}
