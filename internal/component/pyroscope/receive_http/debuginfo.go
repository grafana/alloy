package receive_http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/runtime/logging/level"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfoclient"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
)

func (c *Component) getDebugInfoClients() []*debuginfoclient.Client {
	c.mut.Lock()
	defer c.mut.Unlock()
	var clients []*debuginfoclient.Client
	for _, appendable := range c.appendables {
		clients = append(clients, appendable.DebugInfoClients()...)
	}
	return clients
}

func (c *Component) firstClient() (*debuginfoclient.Client, error) {
	clients := c.getDebugInfoClients()
	if len(clients) == 0 {
		return nil, fmt.Errorf("no downstream endpoints available")
	}
	return clients[0], nil
}

func (c *Component) recordDownstream(l log.Logger, method string, err error) {
	result := "success"
	if err != nil {
		result = "failure"
	}
	c.metrics.debugInfoDownstreamCalls.WithLabelValues(method, result).Inc()
	if err != nil {
		_ = level.Error(l).Log("err", err)
	} else {
		_ = level.Debug(l).Log("result", "ok")
	}
}

func (c *Component) ShouldInitiateUpload(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (res *connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], err error) {
	l := log.With(c.logger,
		"pyroscope_proxy", "debuginfo",
		"method", "ShouldInitiateUpload DS",
		"name", req.Msg.File.Name,
		"gnu_build_id", req.Msg.File.GnuBuildId,
		"go_build_id", req.Msg.File.GoBuildId,
		"otel_file_id", req.Msg.File.OtelFileId,
	)
	defer func() { c.recordDownstream(l, "ShouldInitiateUpload", err) }()

	client, err := c.firstClient()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return client.ShouldInitiateUpload(ctx, connect.NewRequest(req.Msg.CloneVT()))
}

func (c *Component) UploadFinished(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (res *connect.Response[debuginfov1alpha1.UploadFinishedResponse], err error) {
	l := log.With(c.logger,
		"pyroscope_proxy", "debuginfo",
		"method", "UploadFinished DS",
		"gnu_build_id", req.Msg.GnuBuildId,
	)
	defer func() { c.recordDownstream(l, "UploadFinished", err) }()

	client, err := c.firstClient()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return client.UploadFinished(ctx, connect.NewRequest(req.Msg.CloneVT()))
}

func (c *Component) UploadHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gnuBuildID := mux.Vars(r)["gnu_build_id"]
		l := log.With(c.logger,
			"pyroscope_proxy", "debuginfo",
			"method", "Upload DS",
			"gnu_build_id", gnuBuildID,
		)

		var err error
		defer func() { c.recordDownstream(l, "Upload", err) }()

		client, err := c.firstClient()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		c.mut.Lock()
		uploadTimeout := c.debugInfoUploadTimeout
		c.mut.Unlock()

		// Extend server read/write deadlines so the upload is not
		// killed by the default HTTP server timeouts (typically 30s).
		rc := http.NewResponseController(w)
		deadline := time.Now().Add(uploadTimeout)
		_ = rc.SetReadDeadline(deadline)
		_ = rc.SetWriteDeadline(deadline)

		ctx, cancel := context.WithTimeout(r.Context(), uploadTimeout)
		defer cancel()

		err = client.Upload(ctx, gnuBuildID, r.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("downstream upload: %v", err), http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}
