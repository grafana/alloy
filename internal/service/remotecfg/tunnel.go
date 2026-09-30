package remotecfg

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"time"

	tunnelv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/tunnel/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/tunnel/v1/tunnelv1connect"
)

const maxTunnelResponseBodySize = 5 * 1024 * 1024

type tunnelClientFactory func(Arguments) (tunnelv1connect.TunnelServiceClient, error)

// tunnelRunner keeps the active tunnel aligned with the latest remote
// configuration without making tunnel failures fatal to Alloy.
type tunnelRunner struct {
	logger        *slog.Logger
	httpClient    *http.Client
	httpServerURL string
	factory       tunnelClientFactory

	mut     sync.Mutex
	desired Arguments
	updates chan struct{}

	initialBackoff time.Duration
	maxBackoff     time.Duration
}

func newTunnelRunner(logger *slog.Logger, httpClient *http.Client, httpServerURL string, factory tunnelClientFactory) *tunnelRunner {
	return &tunnelRunner{
		logger:         logger,
		httpClient:     httpClient,
		httpServerURL:  httpServerURL,
		factory:        factory,
		updates:        make(chan struct{}, 1),
		initialBackoff: time.Second,
		maxBackoff:     time.Minute,
	}
}

// Update replaces the desired tunnel configuration and wakes the runner when
// a connection-relevant setting changes.
func (r *tunnelRunner) Update(args Arguments) {
	r.mut.Lock()
	changed := r.desired.URL != args.URL ||
		r.desired.TunnelURL != args.TunnelURL ||
		r.desired.ID != args.ID ||
		!reflect.DeepEqual(r.desired.HTTPClientConfig, args.HTTPClientConfig)
	r.desired = args
	r.mut.Unlock()

	if !changed {
		return
	}
	select {
	case r.updates <- struct{}{}:
	default:
	}
}

// Run reconciles configuration updates with at most one reconnect loop.
func (r *tunnelRunner) Run(ctx context.Context) {
	var (
		cancelCurrent context.CancelFunc
		currentDone   <-chan struct{}
	)

	stopCurrent := func() {
		if cancelCurrent == nil {
			return
		}
		cancelCurrent()
		<-currentDone
		cancelCurrent = nil
		currentDone = nil
	}

	for {
		select {
		case <-ctx.Done():
			stopCurrent()
			return
		case <-r.updates:
			stopCurrent()
			for {
				select {
				case <-r.updates:
					continue
				default:
				}
				break
			}

			r.mut.Lock()
			args := r.desired
			r.mut.Unlock()
			if args.URL == "" || r.httpClient == nil || r.httpServerURL == "" {
				continue
			}

			reconnectCtx, cancel := context.WithCancel(ctx)
			done := make(chan struct{})
			cancelCurrent = cancel
			currentDone = done
			go func() {
				defer close(done)
				r.runReconnectLoop(reconnectCtx, args)
			}()
		}
	}
}

// runReconnectLoop maintains one tunnel session and retries failures with
// bounded exponential backoff.
func (r *tunnelRunner) runReconnectLoop(ctx context.Context, args Arguments) {
	backoff := r.initialBackoff
	for {
		r.logger.Info(
			"Establishing Fleet Management tunnel",
			"url", args.getTunnelURL(),
			"collector_id", args.ID,
		)
		tunnelClient, err := r.factory(args)
		established := false
		if err == nil {
			established, err = runTunnelSessionAttempt(ctx, r.logger, tunnelClient, args.ID, r.httpClient, r.httpServerURL)
		}
		if ctx.Err() != nil {
			return
		}
		if established {
			backoff = r.initialBackoff
			r.logger.Warn("Fleet Management tunnel disconnected; reconnecting", "err", err)
		} else {
			r.logger.Warn("Failed to establish Fleet Management tunnel; retrying", "err", err)
		}

		timer := time.NewTimer(jitteredBackoff(backoff))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
		backoff = min(backoff*2, r.maxBackoff)
	}
}

func jitteredBackoff(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	jitterRange := max(delay/10, 1)
	return delay - jitterRange + time.Duration(rand.Int64N(int64(2*jitterRange+1)))
}

func runTunnelSession(
	ctx context.Context,
	logger *slog.Logger,
	client tunnelv1connect.TunnelServiceClient,
	collectorID string,
	httpClient *http.Client,
	httpServerURL string,
) error {

	_, err := runTunnelSessionAttempt(ctx, logger, client, collectorID, httpClient, httpServerURL)
	return err
}

// runTunnelSessionAttempt registers the collector, then processes requests
// synchronously until the stream closes.
func runTunnelSessionAttempt(
	ctx context.Context,
	logger *slog.Logger,
	client tunnelv1connect.TunnelServiceClient,
	collectorID string,
	httpClient *http.Client,
	httpServerURL string,
) (bool, error) {

	stream := client.RegisterCollector(ctx)
	defer func() {
		_ = stream.CloseRequest()
		_ = stream.CloseResponse()
	}()

	stream.RequestHeader().Set("X-Collector-ID", collectorID)
	if err := stream.Send(nil); err != nil {
		return false, fmt.Errorf("send collector registration headers: %w", err)
	}
	logger.Info(
		"Fleet Management tunnel established; collector registered",
		"collector_id", collectorID,
	)

	for {
		message, err := stream.Receive()
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		if err != nil {
			return true, fmt.Errorf("receive server message: %w", err)
		}

		request := message.GetRemoteRequest()
		if request == nil {
			if cancelRequest := message.GetCancelRequest(); cancelRequest != nil {
				logger.Info(
					"Ignoring Fleet Management tunnel cancellation",
					"request_id", cancelRequest.GetRequestId(),
				)
			} else {
				logger.Warn("Ignoring Fleet Management tunnel message with no request")
			}
			continue
		}
		logger.Info(
			"Fleet Management tunnel request received",
			"request_id", request.GetRequestId(),
			"method", request.GetRequest().GetMethod().String(),
			"request_uri", request.GetRequest().GetRequestUri(),
			"content_type", request.GetRequest().GetContentType(),
			"request_size", len(request.GetRequest().GetBody()),
			"request_body", string(request.GetRequest().GetBody()),
		)

		result := handleTunnelRequest(ctx, request, httpClient, httpServerURL)
		if err := stream.Send(&tunnelv1.CollectorMessage{
			Message: &tunnelv1.CollectorMessage_Result{
				Result: result,
			},
		}); err != nil {
			return true, fmt.Errorf("send request result: %w", err)
		}
		logger.Info(
			"Fleet Management tunnel response sent",
			"request_id", result.GetRequestId(),
			"status", result.GetResponse().GetStatus(),
			"response_size", len(result.GetResponse().GetBody()),
		)
	}
}

// handleTunnelRequest validates the narrow tunnel API and forwards accepted
// requests through Alloy's in-memory HTTP server.
func handleTunnelRequest(ctx context.Context, remote *tunnelv1.RemoteRequest, httpClient *http.Client, httpServerURL string) *tunnelv1.RemoteRequestResult {
	result := &tunnelv1.RemoteRequestResult{RequestId: remote.GetRequestId()}
	request := remote.GetRequest()
	if request == nil {
		result.Response = tunnelHTTPResponse(http.StatusBadRequest, "", "missing request")
		return result
	}

	requestURI := request.GetRequestUri()
	parsedURI, err := url.ParseRequestURI(requestURI)
	if err != nil {
		result.Response = tunnelHTTPResponse(http.StatusBadRequest, requestURI, "malformed request URI")
		return result
	}
	if parsedURI.IsAbs() || parsedURI.Host != "" || parsedURI.Path != "/graphql" {
		result.Response = tunnelHTTPResponse(http.StatusNotFound, requestURI, http.StatusText(http.StatusNotFound))
		return result
	}
	if request.GetMethod() != tunnelv1.HTTPRequestMethod_HTTP_REQUEST_METHOD_POST {
		result.Response = tunnelHTTPResponse(http.StatusMethodNotAllowed, requestURI, http.StatusText(http.StatusMethodNotAllowed))
		return result
	}
	if contentType := request.GetContentType(); contentType != "" && contentType != "application/json" {
		result.Response = tunnelHTTPResponse(http.StatusUnsupportedMediaType, requestURI, http.StatusText(http.StatusUnsupportedMediaType))
		return result
	}

	localRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		httpServerURL+requestURI,
		bytes.NewReader(request.GetBody()),
	)
	if err != nil {
		result.Response = tunnelHTTPResponse(http.StatusBadGateway, requestURI, http.StatusText(http.StatusBadGateway))
		return result
	}
	localRequest.Header.Set("Content-Type", "application/json")

	raw, err := httpClient.Do(localRequest)
	if err != nil {
		result.Response = tunnelHTTPResponse(http.StatusBadGateway, requestURI, http.StatusText(http.StatusBadGateway))
		return result
	}
	defer raw.Body.Close()

	body, err := io.ReadAll(io.LimitReader(raw.Body, maxTunnelResponseBodySize+1))
	if err != nil || len(body) > maxTunnelResponseBodySize {
		result.Response = tunnelHTTPResponse(http.StatusBadGateway, requestURI, http.StatusText(http.StatusBadGateway))
		return result
	}
	result.Response = &tunnelv1.HTTPResponse{
		Status:     uint32(raw.StatusCode),
		RequestUri: requestURI,
		Body:       body,
	}
	return result
}

func tunnelHTTPResponse(status int, requestURI, body string) *tunnelv1.HTTPResponse {
	return &tunnelv1.HTTPResponse{
		Status:     uint32(status),
		RequestUri: requestURI,
		Body:       []byte(body),
	}
}
