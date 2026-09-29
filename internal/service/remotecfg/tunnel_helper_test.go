package remotecfg

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	tunnelv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/tunnel/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/tunnel/v1/tunnelv1connect"
	"github.com/grafana/alloy/internal/component/common/config"
	graphqlservice "github.com/grafana/alloy/internal/service/graphql"
	"github.com/stretchr/testify/require"
)

const tunnelTestTimeout = 5 * time.Second

type tunnelTestHarness struct {
	tunnelv1connect.UnimplementedTunnelServiceHandler

	t           *testing.T
	collectorID string
	requests    chan tunnelTestExchange
	hello       chan *tunnelv1.CollectorHelloMessage

	alloyHTTPServer *httptest.Server
	tunnelServer    *httptest.Server
	cancel          context.CancelFunc
	sessionStop     chan struct{}
	sessionDone     chan struct{}
	sessionErr      error

	mut       sync.Mutex
	started   bool
	nextID    uint64
	pendingID uint64
	replies   sync.WaitGroup
}

type tunnelTestExchange struct {
	request *tunnelv1.RemoteRequest
	reply   chan *tunnelv1.RemoteRequestResult
}

func newTunnelTestHarness(t *testing.T) *tunnelTestHarness {
	t.Helper()
	return &tunnelTestHarness{
		t:           t,
		collectorID: "collector-1",
		requests:    make(chan tunnelTestExchange),
		hello:       make(chan *tunnelv1.CollectorHelloMessage, 1),
		sessionStop: make(chan struct{}),
	}
}

func (h *tunnelTestHarness) Start() {
	h.t.Helper()

	h.mut.Lock()
	if h.started {
		h.mut.Unlock()
		h.t.Fatal("tunnel test harness already started")
	}
	h.started = true
	h.mut.Unlock()

	alloyRouter := mux.NewRouter()
	// This models Alloy with --server.http.enable-graphql enabled.
	graphqlservice.RegisterRoutes(graphqlservice.RegisterRoutesParams{
		Router: alloyRouter,
		Host:   fakeHost{},
	})
	h.alloyHTTPServer = httptest.NewServer(alloyRouter)
	h.alloyHTTPServer.Client().Timeout = tunnelTestTimeout

	tunnelPath, tunnelHandler := tunnelv1connect.NewTunnelServiceHandler(h)
	tunnelMux := http.NewServeMux()
	tunnelMux.Handle(tunnelPath, tunnelHandler)
	h.tunnelServer = httptest.NewUnstartedServer(tunnelMux)
	h.tunnelServer.EnableHTTP2 = true
	h.tunnelServer.StartTLS()

	h.t.Cleanup(h.stop)

	httpConfig := config.CloneDefaultHTTPClientConfig()
	httpConfig.TLSConfig.InsecureSkipVerify = true
	args := Arguments{
		URL:              h.tunnelServer.URL,
		ID:               h.collectorID,
		HTTPClientConfig: httpConfig,
	}
	tunnelClient, err := newTunnelClient(args)
	require.NoError(h.t, err)

	ctx, cancel := context.WithCancel(h.t.Context())
	h.cancel = cancel
	h.sessionDone = make(chan struct{})
	go func() {
		defer close(h.sessionDone)
		h.sessionErr = runTunnelSession(
			ctx,
			slog.Default(),
			tunnelClient,
			args.ID,
			h.alloyHTTPServer.Client(),
			h.alloyHTTPServer.URL,
		)
	}()

	select {
	case hello := <-h.hello:
		require.Equal(h.t, h.collectorID, hello.GetCollectorId())
	case <-h.sessionDone:
		require.NoError(h.t, h.sessionErr)
		h.t.Fatal("tunnel closed before Alloy sent its hello")
	case <-time.After(tunnelTestTimeout):
		h.t.Fatal("timed out waiting for Alloy's hello")
	}
}

func (h *tunnelTestHarness) Send(request *tunnelv1.HTTPRequest) (uint64, <-chan *tunnelv1.RemoteRequestResult) {
	h.t.Helper()

	h.mut.Lock()
	if !h.started {
		h.mut.Unlock()
		h.t.Fatal("tunnel test harness is not started")
	}
	if h.pendingID != 0 {
		h.mut.Unlock()
		h.t.Fatalf("request %d is still pending", h.pendingID)
	}
	h.nextID++
	requestID := h.nextID
	h.pendingID = requestID
	h.mut.Unlock()

	streamReply := make(chan *tunnelv1.RemoteRequestResult, 1)
	testReply := make(chan *tunnelv1.RemoteRequestResult, 1)
	exchange := tunnelTestExchange{
		request: &tunnelv1.RemoteRequest{
			RequestId: requestID,
			Request:   request,
		},
		reply: streamReply,
	}

	select {
	case h.requests <- exchange:
	case <-time.After(tunnelTestTimeout):
		h.clearPending(requestID)
		h.t.Fatal("timed out sending tunnel request")
	}

	h.replies.Add(1)
	go h.awaitReply(requestID, streamReply, testReply)
	return requestID, testReply
}

func (h *tunnelTestHarness) awaitReply(
	requestID uint64,
	streamReply <-chan *tunnelv1.RemoteRequestResult,
	testReply chan<- *tunnelv1.RemoteRequestResult,
) {

	defer h.replies.Done()
	defer close(testReply)
	defer h.clearPending(requestID)

	select {
	case result, ok := <-streamReply:
		if ok {
			testReply <- result
		}
	case <-time.After(tunnelTestTimeout):
		h.t.Errorf("timed out waiting for response to tunnel request %d", requestID)
	case <-h.t.Context().Done():
		h.t.Errorf("test ended before tunnel request %d received a response", requestID)
	}
}

func (h *tunnelTestHarness) RegisterCollector(ctx context.Context, stream *connect.BidiStream[tunnelv1.CollectorMessage, tunnelv1.ServerMessage]) error {
	message, err := stream.Receive()
	if err != nil {
		return err
	}
	if message.GetHello() == nil {
		return fmt.Errorf("first collector message was not a hello")
	}
	h.hello <- message.GetHello()

	for {
		select {
		case exchange := <-h.requests:
			if err := sendTunnelTestRequest(stream, exchange.request); err != nil {
				close(exchange.reply)
				return err
			}

			message, err := stream.Receive()
			if err != nil {
				close(exchange.reply)
				return err
			}
			if message.GetResult() == nil {
				close(exchange.reply)
				return fmt.Errorf("collector message was not a result")
			}
			if message.GetResult().GetRequestId() != exchange.request.GetRequestId() {
				h.t.Errorf(
					"response request ID %d does not match request ID %d",
					message.GetResult().GetRequestId(),
					exchange.request.GetRequestId(),
				)
			}
			exchange.reply <- message.GetResult()
			close(exchange.reply)
		case <-ctx.Done():
			return ctx.Err()
		case <-h.sessionStop:
			return nil
		}
	}
}

func (h *tunnelTestHarness) clearPending(requestID uint64) {
	h.mut.Lock()
	defer h.mut.Unlock()
	if h.pendingID == requestID {
		h.pendingID = 0
	}
}

func (h *tunnelTestHarness) stop() {
	close(h.sessionStop)
	if h.cancel != nil {
		h.cancel()
	}
	if h.sessionDone != nil {
		select {
		case <-h.sessionDone:
		case <-time.After(tunnelTestTimeout):
			h.t.Error("timed out stopping tunnel session")
		}
	}
	if h.tunnelServer != nil {
		h.tunnelServer.Close()
	}
	if h.alloyHTTPServer != nil {
		h.alloyHTTPServer.Close()
	}
	h.replies.Wait()
}

func sendTunnelTestRequest(
	stream *connect.BidiStream[tunnelv1.CollectorMessage, tunnelv1.ServerMessage],
	request *tunnelv1.RemoteRequest,
) error {

	return stream.Send(&tunnelv1.ServerMessage{
		Message: &tunnelv1.ServerMessage_Request{Request: request},
	})
}

func graphQLRequest(body string) *tunnelv1.HTTPRequest {
	contentType := "application/json"
	return &tunnelv1.HTTPRequest{
		Method:      tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_POST,
		RequestUri:  "/graphql",
		ContentType: &contentType,
		Body:        []byte(body),
	}
}
