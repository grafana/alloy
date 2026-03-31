package receive_http

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
)

// mockDebuginfoHandler implements debuginfov1alpha1connect.DebuginfoServiceHandler for testing.
type mockDebuginfoHandler struct {
	uploadFunc func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error
}

func (m *mockDebuginfoHandler) Upload(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
	return m.uploadFunc(ctx, stream)
}

// downstreamResult captures what a mock downstream server received.
type downstreamResult struct {
	initFile *debuginfov1alpha1.FileMetadata
	data     []byte
	chunks   int
}

// startMockDownstream creates a TLS httptest server (HTTP/2) running a mock debuginfo handler.
// shouldUpload controls whether the server accepts uploads.
// resultCh receives the captured result after the handler finishes.
func startMockDownstream(t *testing.T, shouldUpload bool, resultCh chan<- downstreamResult) debuginfov1alpha1connect.DebuginfoServiceClient {
	t.Helper()
	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			req, err := stream.Receive()
			if err != nil {
				return err
			}

			if err := stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: shouldUpload,
					},
				},
			}); err != nil {
				return err
			}

			res := downstreamResult{
				initFile: req.GetInit().GetFile(),
			}

			if shouldUpload {
				for {
					chunkReq, err := stream.Receive()
					if err != nil {
						break
					}
					if chunk := chunkReq.GetChunk(); chunk != nil {
						res.chunks++
						res.data = append(res.data, chunk.GetChunk()...)
					}
				}
			}

			resultCh <- res
			return nil
		},
	}

	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(handler)
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	return debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
}

// debuginfoAppendable is a test pyroscope.Appendable that returns Connect debuginfo clients.
type debuginfoAppendable struct {
	clients []debuginfov1alpha1connect.DebuginfoServiceClient
}

func (d *debuginfoAppendable) Appender() pyroscope.Appender { return d }
func (d *debuginfoAppendable) Append(_ context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
	return nil
}
func (d *debuginfoAppendable) AppendIngest(_ context.Context, _ *pyroscope.IncomingProfile) error {
	return nil
}
func (d *debuginfoAppendable) Upload(_ debuginfo.UploadJob) {}
func (d *debuginfoAppendable) DebugInfoClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	return d.clients
}

// startProxyServer creates a Component with the given appendables and serves its
// debuginfo Upload handler via an HTTP/2 TLS test server. Returns a Connect client.
func startProxyServer(t *testing.T, appendables []pyroscope.Appendable) debuginfov1alpha1connect.DebuginfoServiceClient {
	t.Helper()
	comp := &Component{
		appendables: appendables,
	}
	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(comp)
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	return debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
}

// sendUploadViaProxy opens a bidi stream to the proxy client, sends an init request, reads the
// init response, and if accepted, streams fileData as chunks.
func sendUploadViaProxy(t *testing.T, proxyClient debuginfov1alpha1connect.DebuginfoServiceClient, fileData []byte) (bool, error) {
	t.Helper()
	stream := proxyClient.Upload(context.Background())

	// Send init.
	err := stream.Send(&debuginfov1alpha1.UploadRequest{
		Data: &debuginfov1alpha1.UploadRequest_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadRequest{
				File: &debuginfov1alpha1.FileMetadata{
					GnuBuildId: "test-build-id",
					Name:       "test.so",
					Type:       debuginfov1alpha1.FileMetadata_TYPE_EXECUTABLE_FULL,
				},
			},
		},
	})
	if err != nil {
		return false, fmt.Errorf("send init: %w", err)
	}

	// Receive init response.
	resp, err := stream.Receive()
	if err != nil {
		return false, fmt.Errorf("receive init response: %w", err)
	}

	initResp := resp.GetInit()
	if initResp == nil {
		return false, fmt.Errorf("expected init response")
	}

	if !initResp.ShouldInitiateUpload {
		_ = stream.CloseRequest()
		return false, nil
	}

	// Stream chunks.
	chunkSize := 1024
	for offset := 0; offset < len(fileData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		if err := stream.Send(&debuginfov1alpha1.UploadRequest{
			Data: &debuginfov1alpha1.UploadRequest_Chunk{
				Chunk: &debuginfov1alpha1.UploadChunk{
					Chunk: fileData[offset:end],
				},
			},
		}); err != nil {
			return true, fmt.Errorf("send chunk: %w", err)
		}
	}

	_ = stream.CloseRequest()
	return true, nil
}

func TestDebugInfoProxy_SingleEndpoint_AcceptsUpload(t *testing.T) {
	resultCh := make(chan downstreamResult, 1)
	dsClient := startMockDownstream(t, true, resultCh)

	appendable := &debuginfoAppendable{clients: []debuginfov1alpha1connect.DebuginfoServiceClient{dsClient}}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("hello proxy debuginfo upload test data")
	accepted, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.NoError(t, err)
	require.True(t, accepted)

	select {
	case res := <-resultCh:
		require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId())
		require.Equal(t, "test.so", res.initFile.GetName())
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for downstream to receive upload")
	}
}

func TestDebugInfoProxy_MultipleEndpoints_AllAccept(t *testing.T) {
	const numEndpoints = 3
	resultChs := make([]chan downstreamResult, numEndpoints)
	clients := make([]debuginfov1alpha1connect.DebuginfoServiceClient, numEndpoints)

	for i := 0; i < numEndpoints; i++ {
		resultChs[i] = make(chan downstreamResult, 1)
		clients[i] = startMockDownstream(t, true, resultChs[i])
	}

	appendable := &debuginfoAppendable{clients: clients}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("multi-endpoint-test-data-for-all-accepting-servers")
	accepted, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.NoError(t, err)
	require.True(t, accepted)

	for i := 0; i < numEndpoints; i++ {
		select {
		case res := <-resultChs[i]:
			require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId(), "endpoint %d", i)
			require.Equal(t, fileData, res.data, "endpoint %d data mismatch", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for downstream %d", i)
		}
	}
}

func TestDebugInfoProxy_MultipleEndpoints_AllDecline(t *testing.T) {
	const numEndpoints = 3
	resultChs := make([]chan downstreamResult, numEndpoints)
	clients := make([]debuginfov1alpha1connect.DebuginfoServiceClient, numEndpoints)

	for i := 0; i < numEndpoints; i++ {
		resultChs[i] = make(chan downstreamResult, 1)
		clients[i] = startMockDownstream(t, false, resultChs[i])
	}

	appendable := &debuginfoAppendable{clients: clients}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("should-not-be-sent")
	accepted, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.NoError(t, err)
	require.False(t, accepted, "proxy should decline when all endpoints decline")

	// Verify all downstreams received the init but no chunks.
	for i := 0; i < numEndpoints; i++ {
		select {
		case res := <-resultChs[i]:
			require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId(), "endpoint %d", i)
			require.Nil(t, res.data, "endpoint %d should not have received chunks", i)
			require.Equal(t, 0, res.chunks, "endpoint %d should not have received chunks", i)
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for downstream %d", i)
		}
	}
}

func TestDebugInfoProxy_MultipleEndpoints_SomeAccept(t *testing.T) {
	// 3 endpoints: [0]=decline, [1]=accept, [2]=accept
	accepts := []bool{false, true, true}
	resultChs := make([]chan downstreamResult, len(accepts))
	clients := make([]debuginfov1alpha1connect.DebuginfoServiceClient, len(accepts))

	for i, shouldAccept := range accepts {
		resultChs[i] = make(chan downstreamResult, 1)
		clients[i] = startMockDownstream(t, shouldAccept, resultChs[i])
	}

	appendable := &debuginfoAppendable{clients: clients}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("partial-accept-test-data")
	accepted, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.NoError(t, err)
	require.True(t, accepted, "proxy should accept when at least one endpoint accepts")

	for i, shouldAccept := range accepts {
		select {
		case res := <-resultChs[i]:
			require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId(), "endpoint %d", i)
			if shouldAccept {
				require.Equal(t, fileData, res.data, "accepting endpoint %d data mismatch", i)
			} else {
				require.Nil(t, res.data, "declining endpoint %d should not receive chunks", i)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for downstream %d", i)
		}
	}
}

func TestDebugInfoProxy_NoEndpoints(t *testing.T) {
	// No downstream clients at all.
	appendable := &debuginfoAppendable{clients: nil}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})
	stream := proxyClient.Upload(context.Background())

	err := stream.Send(&debuginfov1alpha1.UploadRequest{
		Data: &debuginfov1alpha1.UploadRequest_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadRequest{
				File: &debuginfov1alpha1.FileMetadata{
					GnuBuildId: "test",
					Name:       "test.so",
				},
			},
		},
	})
	if err != nil {
		// Error on send is acceptable — server may reject immediately.
		return
	}

	_, err = stream.Receive()
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}

func TestDebugInfoProxy_InvalidFirstMessage_ReturnsError(t *testing.T) {
	resultCh := make(chan downstreamResult, 1)
	dsClient := startMockDownstream(t, true, resultCh)

	appendable := &debuginfoAppendable{clients: []debuginfov1alpha1connect.DebuginfoServiceClient{dsClient}}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	stream := proxyClient.Upload(context.Background())

	// Send a chunk as the first message instead of init.
	err := stream.Send(&debuginfov1alpha1.UploadRequest{
		Data: &debuginfov1alpha1.UploadRequest_Chunk{
			Chunk: &debuginfov1alpha1.UploadChunk{
				Chunk: []byte("bad"),
			},
		},
	})
	if err != nil {
		return // server rejected on send, acceptable
	}

	_, err = stream.Receive()
	require.Error(t, err)
	require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestDebugInfoProxy_DownstreamRespondsWithNonInit(t *testing.T) {
	// Downstream responds to the init request with a non-init message.
	// The proxy must not panic on nil GetInit().
	badHandler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			_, err := stream.Receive()
			if err != nil {
				return err
			}
			// Send an empty response (no Init field set).
			return stream.Send(&debuginfov1alpha1.UploadResponse{})
		},
	}
	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(badHandler)
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	badClient := debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)

	appendable := &debuginfoAppendable{
		clients: []debuginfov1alpha1connect.DebuginfoServiceClient{badClient},
	}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("should-not-be-sent")
	_, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.Error(t, err, "proxy should return error when downstream sends invalid init response")
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestDebugInfoProxy_FailClosesStream(t *testing.T) {
	// Two downstreams: bad (index 0) sends a non-init response then blocks on Receive.
	// Good (index 1) waits for badDone before sending its accept response.
	//
	// If fail() closes the stream: bad handler unblocks → badDone fires → good handler
	// proceeds → proxy gets accept → test passes.
	// If fail() doesn't close: bad handler stays stuck → badDone never fires →
	// good handler blocks → proxy blocks on Receive → test times out.
	badDone := make(chan struct{})
	badHandler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			defer close(badDone)
			if _, err := stream.Receive(); err != nil {
				return err
			}
			if err := stream.Send(&debuginfov1alpha1.UploadResponse{}); err != nil {
				return err
			}
			// Block until stream is closed.
			_, _ = stream.Receive()
			return nil
		},
	}
	_, badH := debuginfov1alpha1connect.NewDebuginfoServiceHandler(badHandler)
	badServer := httptest.NewUnstartedServer(badH)
	badServer.EnableHTTP2 = true
	badServer.StartTLS()
	t.Cleanup(badServer.Close)
	badClient := debuginfov1alpha1connect.NewDebuginfoServiceClient(badServer.Client(), badServer.URL)

	goodResultCh := make(chan downstreamResult, 1)
	goodHandler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			req, err := stream.Receive()
			if err != nil {
				return err
			}
			// Wait for bad handler to be unblocked before responding.
			select {
			case <-badDone:
			case <-ctx.Done():
				return ctx.Err()
			}
			if err := stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: false,
					},
				},
			}); err != nil {
				return err
			}
			goodResultCh <- downstreamResult{initFile: req.GetInit().GetFile()}
			return nil
		},
	}
	_, goodH := debuginfov1alpha1connect.NewDebuginfoServiceHandler(goodHandler)
	goodServer := httptest.NewUnstartedServer(goodH)
	goodServer.EnableHTTP2 = true
	goodServer.StartTLS()
	t.Cleanup(goodServer.Close)
	goodClient := debuginfov1alpha1connect.NewDebuginfoServiceClient(goodServer.Client(), goodServer.URL)

	// Bad client must be first so the proxy processes it before good.
	appendable := &debuginfoAppendable{
		clients: []debuginfov1alpha1connect.DebuginfoServiceClient{badClient, goodClient},
	}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("should-not-be-sent")
	_, err := sendUploadViaProxy(t, proxyClient, fileData)
	// The bad downstream's init failure now causes the proxy to return an error
	// instead of a clean decline, but the key assertion is that we don't time out:
	// fail() must close the bad stream so badDone fires and the good handler unblocks.
	require.Error(t, err)
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

func TestDebugInfoProxy_DownstreamFailsDuringInit_ReturnsError(t *testing.T) {
	// One downstream errors immediately during init, one declines normally.
	// The proxy should return an error (not a clean decline) so the caller
	// retries instead of permanently caching the file ID.
	crashHandler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			return connect.NewError(connect.CodeInternal, fmt.Errorf("init crash"))
		},
	}
	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(crashHandler)
	crashServer := httptest.NewUnstartedServer(h)
	crashServer.EnableHTTP2 = true
	crashServer.StartTLS()
	t.Cleanup(crashServer.Close)
	crashClient := debuginfov1alpha1connect.NewDebuginfoServiceClient(crashServer.Client(), crashServer.URL)

	declineResultCh := make(chan downstreamResult, 1)
	declineClient := startMockDownstream(t, false, declineResultCh)

	appendable := &debuginfoAppendable{
		clients: []debuginfov1alpha1connect.DebuginfoServiceClient{crashClient, declineClient},
	}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("should-not-be-sent")
	_, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.Error(t, err, "proxy should return an error when any downstream failed during init")
	require.Equal(t, connect.CodeInternal, connect.CodeOf(err))
}

// startMockDownstreamCrashAfter creates a downstream that accepts upload but returns an error
// after receiving crashAfter chunks, simulating a mid-stream failure.
func startMockDownstreamCrashAfter(t *testing.T, crashAfter int, resultCh chan<- downstreamResult) debuginfov1alpha1connect.DebuginfoServiceClient {
	t.Helper()
	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			req, err := stream.Receive()
			if err != nil {
				return err
			}

			if err := stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: true,
					},
				},
			}); err != nil {
				return err
			}

			res := downstreamResult{
				initFile: req.GetInit().GetFile(),
			}

			for i := 0; i < crashAfter; i++ {
				chunkReq, err := stream.Receive()
				if err != nil {
					break
				}
				if chunk := chunkReq.GetChunk(); chunk != nil {
					res.chunks++
					res.data = append(res.data, chunk.GetChunk()...)
				}
			}

			resultCh <- res
			// Return error to simulate crash — this closes the stream abruptly.
			return connect.NewError(connect.CodeInternal, fmt.Errorf("simulated downstream crash"))
		},
	}

	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(handler)
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	return debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
}

func TestDebugInfoProxy_DownstreamFailsMidStream_HealthyGetsAllData(t *testing.T) {
	// Set up 2 endpoints: [0] crashes after 1 chunk, [1] is healthy and receives all data.
	crashResultCh := make(chan downstreamResult, 1)
	crashClient := startMockDownstreamCrashAfter(t, 1, crashResultCh)

	healthyResultCh := make(chan downstreamResult, 1)
	healthyClient := startMockDownstream(t, true, healthyResultCh)

	appendable := &debuginfoAppendable{
		clients: []debuginfov1alpha1connect.DebuginfoServiceClient{crashClient, healthyClient},
	}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	// Send 3KB of data (3 chunks of 1KB each via sendUploadViaProxy).
	fileData := make([]byte, 3*1024)
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	accepted, err := sendUploadViaProxy(t, proxyClient, fileData)
	require.NoError(t, err)
	require.True(t, accepted)

	// The crashing endpoint should have received only 1 chunk.
	select {
	case res := <-crashResultCh:
		require.Equal(t, 1, res.chunks, "crashing endpoint should have received 1 chunk before crash")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for crashing downstream")
	}

	// The healthy endpoint should have received ALL the data.
	select {
	case res := <-healthyResultCh:
		require.Equal(t, fileData, res.data, "healthy endpoint should receive all data despite other endpoint crashing")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for healthy downstream")
	}
}

func TestDebugInfoProxy_AllDownstreamsFailMidStream_ReturnsError(t *testing.T) {
	// Single downstream that crashes after 1 chunk.
	// The proxy should return an error when all downstreams have failed.
	crashResultCh := make(chan downstreamResult, 1)
	crashClient := startMockDownstreamCrashAfter(t, 1, crashResultCh)

	appendable := &debuginfoAppendable{
		clients: []debuginfov1alpha1connect.DebuginfoServiceClient{crashClient},
	}
	proxyClient := startProxyServer(t, []pyroscope.Appendable{appendable})

	// Send 3KB of data (3 chunks of 1KB each).
	fileData := make([]byte, 3*1024)
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	stream := proxyClient.Upload(context.Background())

	// Send init.
	err := stream.Send(&debuginfov1alpha1.UploadRequest{
		Data: &debuginfov1alpha1.UploadRequest_Init{
			Init: &debuginfov1alpha1.ShouldInitiateUploadRequest{
				File: &debuginfov1alpha1.FileMetadata{
					GnuBuildId: "test-build-id",
					Name:       "test.so",
					Type:       debuginfov1alpha1.FileMetadata_TYPE_EXECUTABLE_FULL,
				},
			},
		},
	})
	require.NoError(t, err)

	// Receive init response — should accept.
	resp, err := stream.Receive()
	require.NoError(t, err)
	require.True(t, resp.GetInit().GetShouldInitiateUpload())

	// Stream chunks.
	chunkSize := 1024
	for offset := 0; offset < len(fileData); offset += chunkSize {
		end := offset + chunkSize
		if end > len(fileData) {
			end = len(fileData)
		}
		_ = stream.Send(&debuginfov1alpha1.UploadRequest{
			Data: &debuginfov1alpha1.UploadRequest_Chunk{
				Chunk: &debuginfov1alpha1.UploadChunk{
					Chunk: fileData[offset:end],
				},
			},
		})
	}
	_ = stream.CloseRequest()

	// Wait for crash downstream to process.
	select {
	case <-crashResultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for crashing downstream")
	}

	// The proxy should have returned an error since all downstreams failed.
	_, err = stream.Receive()
	require.Error(t, err, "proxy should return an error when all downstream streams failed mid-upload")
}
