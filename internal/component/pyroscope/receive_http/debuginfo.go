package receive_http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"connectrpc.com/connect"
	pyroutil "github.com/grafana/alloy/internal/component/pyroscope/util"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"google.golang.org/protobuf/proto"
)

func (c *Component) getDebugInfoClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	c.mut.Lock()
	defer c.mut.Unlock()
	var clients []debuginfov1alpha1connect.DebuginfoServiceClient
	for _, appendable := range c.appendables {
		clients = append(clients, appendable.DebugInfoClients()...)
	}
	return clients
}

type debuginfoDownstream struct {
	stream *connect.BidiStreamForClient[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]
	failed bool
	errs   error
	errMut sync.Mutex
}

func (d *debuginfoDownstream) fail(err error) {
	d.failed = true
	_ = d.stream.CloseRequest()
	_ = d.stream.CloseResponse()
	pyroutil.ErrorsJoinConcurrent(&d.errs, err, &d.errMut)
}

func debuginfoInitReason(i int, accepted bool, reason string) string {
	if reason != "" {
		return reason
	}
	if accepted {
		return fmt.Sprintf("downstream %d accepted", i)
	}
	return fmt.Sprintf("downstream %d declined", i)
}

func (c *Component) Upload(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
	clients := c.getDebugInfoClients()
	if len(clients) == 0 {
		return connect.NewError(connect.CodeUnavailable, fmt.Errorf("no downstream endpoints available"))
	}

	initReq, err := stream.Receive()
	if err != nil {
		return err
	}
	if initReq.GetInit() == nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("expected init request, got %T", initReq.GetData()))
	}

	allDownstreams := make([]debuginfoDownstream, len(clients))
	var accepted []*debuginfoDownstream
	var reasons []string

	for i, client := range clients {
		ds := &allDownstreams[i]
		ds.stream = client.Upload(ctx)

		clonedReq := proto.Clone(initReq).(*debuginfov1alpha1.UploadRequest)
		if err := ds.stream.Send(clonedReq); err != nil {
			ds.fail(fmt.Errorf("downstream %d init send: %w", i, err))
			continue
		}

		resp, err := ds.stream.Receive()
		if err != nil {
			ds.fail(fmt.Errorf("downstream %d init receive: %w", i, err))
			continue
		}

		initResp := resp.GetInit()
		if initResp == nil {
			ds.fail(fmt.Errorf("downstream %d: expected init response, got %T", i, resp.GetData()))
			continue
		}
		if initResp.ShouldInitiateUpload {
			accepted = append(accepted, ds)
		}
		reasons = append(reasons, debuginfoInitReason(i, initResp.ShouldInitiateUpload, initResp.Reason))
	}

	anyAccepted := len(accepted) > 0

	closeAll := func() {
		for i := range allDownstreams {
			_ = allDownstreams[i].stream.CloseRequest()
			_ = allDownstreams[i].stream.CloseResponse()
		}
	}
	defer closeAll()

	// If no downstream accepted, check whether any failed with transport errors.
	// Returning a decline in that case would cause the caller to permanently cache
	// the file ID as "done", preventing retries after a transient outage.
	if !anyAccepted {
		var initErrs error
		for i := range allDownstreams {
			initErrs = errors.Join(initErrs, allDownstreams[i].errs)
		}
		if initErrs != nil {
			return connect.NewError(connect.CodeInternal, initErrs)
		}
	}

	if err := stream.Send(&debuginfov1alpha1.UploadResponse{
		Data: &debuginfov1alpha1.UploadResponse_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: anyAccepted,
				Reason:               strings.Join(reasons, "; "),
			},
		},
	}); err != nil {
		return err
	}

	if !anyAccepted {
		return nil
	}

	aliveCount := len(accepted)

	for aliveCount > 0 {
		req, err := stream.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}

		for _, ds := range accepted {
			if ds.failed {
				continue
			}
			cloned := proto.Clone(req).(*debuginfov1alpha1.UploadRequest)
			if err := ds.stream.Send(cloned); err != nil {
				ds.fail(fmt.Errorf("downstream chunk send: %w", err))
				aliveCount--
			}
		}
	}

	for _, ds := range accepted {
		if ds.failed {
			continue
		}
		_ = ds.stream.CloseRequest()
		for {
			_, err := ds.stream.Receive()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					ds.fail(fmt.Errorf("downstream drain: %w", err))
				}
				break
			}
		}
		_ = ds.stream.CloseResponse()
	}

	var errs error
	for i := range allDownstreams {
		errs = errors.Join(errs, allDownstreams[i].errs)
	}
	if errs != nil {
		return connect.NewError(connect.CodeInternal, errs)
	}
	return nil
}
