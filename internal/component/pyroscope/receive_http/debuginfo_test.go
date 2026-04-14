package receive_http

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/gorilla/mux"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/component/pyroscope/write"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
)

type downstreamResult struct {
	initFile *debuginfov1alpha1.FileMetadata
	data     []byte
}


type mockDebuginfoHandler struct {
	shouldInitiateFunc func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error)
	uploadFinishedFunc func(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error)
}

func (m *mockDebuginfoHandler) ShouldInitiateUpload(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
	return m.shouldInitiateFunc(ctx, req)
}

func (m *mockDebuginfoHandler) UploadFinished(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error) {
	return m.uploadFinishedFunc(ctx, req)
}

func startMockDownstream(t *testing.T, shouldUpload bool, resultCh chan<- downstreamResult) debuginfo.Client {
	t.Helper()
	handler := &mockDebuginfoHandler{
		shouldInitiateFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
			if !shouldUpload {
				resultCh <- downstreamResult{initFile: req.Msg.File}
			}
			return connect.NewResponse(&debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: shouldUpload,
			}), nil
		},
		uploadFinishedFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error) {
			return connect.NewResponse(&debuginfov1alpha1.UploadFinishedResponse{}), nil
		},
	}
	uploadHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		resultCh <- downstreamResult{data: data}
		w.WriteHeader(http.StatusOK)
	})

	router := mux.NewRouter()
	debuginfov1alpha1connect.RegisterDebuginfoServiceHandler(router, handler)
	router.Handle("/debuginfo.v1alpha1.DebuginfoService/Upload/{gnu_build_id}", uploadHTTP).Methods("POST")
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	connectClient := debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
	return &write.DebugInfoClient{
		DebuginfoServiceClient: connectClient,
		HTTPClient:             server.Client(),
		BaseURL:                server.URL,
	}
}

type debuginfoAppendable struct {
	clients []debuginfo.Client
}

func (d *debuginfoAppendable) Appender() pyroscope.Appender { return d }
func (d *debuginfoAppendable) Append(_ context.Context, _ labels.Labels, _ []*pyroscope.RawSample) error {
	return nil
}
func (d *debuginfoAppendable) AppendIngest(_ context.Context, _ *pyroscope.IncomingProfile) error {
	return nil
}
func (d *debuginfoAppendable) Upload(_ debuginfo.UploadJob) {}
func (d *debuginfoAppendable) DebugInfoClients() []debuginfo.Client {
	return d.clients
}

func startProxyServer(t *testing.T, appendables []pyroscope.Appendable) (debuginfov1alpha1connect.DebuginfoServiceClient, *httptest.Server) {
	t.Helper()
	comp := &Component{
		appendables: appendables,
	}
	router := mux.NewRouter()
	debuginfov1alpha1connect.RegisterDebuginfoServiceHandler(router, comp)
	router.Handle("/debuginfo.v1alpha1.DebuginfoService/Upload/{gnu_build_id}", comp.UploadHTTPHandler()).Methods("POST")
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	client := debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
	return client, server
}

func sendUploadViaProxy(t *testing.T, client debuginfov1alpha1connect.DebuginfoServiceClient, srv *httptest.Server, fileData []byte) (bool, error) {
	t.Helper()
	ctx := context.Background()

	resp, err := client.ShouldInitiateUpload(ctx, connect.NewRequest(&debuginfov1alpha1.ShouldInitiateUploadRequest{
		File: &debuginfov1alpha1.FileMetadata{
			GnuBuildId: "test-build-id",
			Name:       "test.so",
			Type:       debuginfov1alpha1.FileMetadata_TYPE_EXECUTABLE_FULL,
		},
	}))
	if err != nil {
		return false, err
	}
	if !resp.Msg.ShouldInitiateUpload {
		return false, nil
	}

	req, err := http.NewRequest("POST", srv.URL+"/debuginfo.v1alpha1.DebuginfoService/Upload/test-build-id", bytes.NewReader(fileData))
	if err != nil {
		return true, err
	}
	httpResp, err := srv.Client().Do(req)
	if err != nil {
		return true, err
	}
	io.Copy(io.Discard, httpResp.Body)
	httpResp.Body.Close()

	_, err = client.UploadFinished(ctx, connect.NewRequest(&debuginfov1alpha1.UploadFinishedRequest{
		GnuBuildId: "test-build-id",
	}))
	return true, err
}

func TestDebugInfoProxy_AcceptsUpload(t *testing.T) {
	resultCh := make(chan downstreamResult, 2)
	dsClient := startMockDownstream(t, true, resultCh)

	appendable := &debuginfoAppendable{clients: []debuginfo.Client{dsClient}}
	client, srv := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("hello proxy debuginfo upload test data")
	accepted, err := sendUploadViaProxy(t, client, srv, fileData)
	require.NoError(t, err)
	require.True(t, accepted)

	select {
	case res := <-resultCh:
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for downstream to receive upload")
	}
}

func TestDebugInfoProxy_DeclinesUpload(t *testing.T) {
	resultCh := make(chan downstreamResult, 1)
	dsClient := startMockDownstream(t, false, resultCh)

	appendable := &debuginfoAppendable{clients: []debuginfo.Client{dsClient}}
	client, srv := startProxyServer(t, []pyroscope.Appendable{appendable})

	fileData := []byte("should-not-be-sent")
	accepted, err := sendUploadViaProxy(t, client, srv, fileData)
	require.NoError(t, err)
	require.False(t, accepted)

	select {
	case res := <-resultCh:
		require.Equal(t, "test-build-id", res.initFile.GetGnuBuildId())
		require.Nil(t, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for downstream")
	}
}

func TestDebugInfoProxy_NoEndpoints(t *testing.T) {
	appendable := &debuginfoAppendable{clients: nil}
	client, _ := startProxyServer(t, []pyroscope.Appendable{appendable})

	_, err := client.ShouldInitiateUpload(context.Background(), connect.NewRequest(&debuginfov1alpha1.ShouldInitiateUploadRequest{
		File: &debuginfov1alpha1.FileMetadata{
			GnuBuildId: "test",
			Name:       "test.so",
		},
	}))
	require.Error(t, err)
	require.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
}
