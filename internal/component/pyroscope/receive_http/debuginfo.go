package receive_http

import (
	"context"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"

	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
)

func (c *Component) getDebugInfoClients() []debuginfo.DebugInfoClient {
	c.mut.Lock()
	defer c.mut.Unlock()
	var clients []debuginfo.DebugInfoClient
	for _, appendable := range c.appendables {
		clients = append(clients, appendable.DebugInfoClients()...)
	}
	return clients
}

func (c *Component) firstClient() (debuginfo.DebugInfoClient, error) {
	clients := c.getDebugInfoClients()
	if len(clients) == 0 {
		return nil, fmt.Errorf("no downstream endpoints available")
	}
	return clients[0], nil
}

func (c *Component) ShouldInitiateUpload(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
	client, err := c.firstClient()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return client.ShouldInitiateUpload(ctx, req)
}

func (c *Component) UploadFinished(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error) {
	client, err := c.firstClient()
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return client.UploadFinished(ctx, req)
}

func (c *Component) UploadHTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client, err := c.firstClient()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		gnuBuildID := mux.Vars(r)["gnu_build_id"]
		if err := client.Upload(r.Context(), gnuBuildID, r.Body); err != nil {
			http.Error(w, fmt.Sprintf("downstream upload: %v", err), http.StatusBadGateway)
			return
		}

		w.WriteHeader(http.StatusOK)
	})
}

