package util

import (
	"math"
	"net/http"
	"net/http/httptest"

	"github.com/grafana/loki/pkg/push"
)

// RemoteWriteRequest wraps the received logs remote write request that is received.
type RemoteWriteRequest struct {
	TenantID string
	Request  push.PushRequest
}

// NewRemoteWriteServer creates and starts a new httpserver.Server that can handle remote write request. When a request is handled,
// the received entries are written to receivedChan, and status is responded.
func NewRemoteWriteServer(receivedChan chan RemoteWriteRequest, status int) *httptest.Server {
	server := httptest.NewServer(createServerHandler(receivedChan, status))
	return server
}

func createServerHandler(receivedReqsChan chan RemoteWriteRequest, receivedOKStatus int) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// Parse the request
		var pushReq push.PushRequest
		if err := ParseProtoReader(req.Context(), req.Body, int(req.ContentLength), math.MaxInt32, &pushReq, RawSnappy); err != nil {
			rw.WriteHeader(500)
			return
		}

		receivedReqsChan <- RemoteWriteRequest{
			TenantID: req.Header.Get("X-Scope-OrgID"),
			Request:  pushReq,
		}

		rw.WriteHeader(receivedOKStatus)
	}
}
