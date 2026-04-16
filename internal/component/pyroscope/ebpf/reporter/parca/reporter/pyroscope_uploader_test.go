//go:build linux && (arm64 || amd64)

package reporter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	"github.com/gorilla/mux"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.uber.org/atomic"
)

type mockReadAtCloser struct {
	*bytes.Reader
	size int64
}

func (m *mockReadAtCloser) Close() error { return nil }

func (m *mockReadAtCloser) Stat() (os.FileInfo, error) {
	return &mockFileInfo{size: m.size}, nil
}

type mockFileInfo struct {
	os.FileInfo
	size int64
}

func (m *mockFileInfo) Size() int64  { return m.size }
func (m *mockFileInfo) IsDir() bool  { return false }
func (m *mockFileInfo) Name() string { return "mock" }

func newMockReadAtCloser(data []byte) func() (process.ReadAtCloser, error) {
	return func() (process.ReadAtCloser, error) {
		return &mockReadAtCloser{
			Reader: bytes.NewReader(data),
			size:   int64(len(data)),
		}, nil
	}
}

func newTestUploader(t *testing.T) (*PyroscopeSymbolUploader, prometheus.Counter) {
	t.Helper()
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_upload_bytes"})
	u, err := NewPyroscopeSymbolUploader(
		log.NewNopLogger(),
		1024,  // cacheSize
		false, // stripTextSection
		64,    // queueSize
		1,     // workerNum
		t.TempDir(),
		counter,
	)
	require.NoError(t, err)
	return u, counter
}

func counterValue(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	_ = c.(prometheus.Metric).Write(m)
	return m.GetCounter().GetValue()
}

type uploadResult struct {
	buildID string
	data    []byte
}

// TODO remove this, do not mERGE
// testDebugInfoClient implements DebugInfoClient by embedding a connect client
// and doing the HTTP POST upload via an httpClient + baseURL.
type testDebugInfoClient struct {
	debuginfov1alpha1connect.DebuginfoServiceClient
	httpClient *http.Client
	baseURL    string
}

func (c *testDebugInfoClient) Upload(ctx context.Context, buildID string, body io.Reader) error {
	uploadURL := strings.TrimRight(c.baseURL, "/") + "/debuginfo.v1alpha1.DebuginfoService/Upload/" + buildID
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, body)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload: HTTP %d", resp.StatusCode)
	}
	return nil
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

func startMockServer(t *testing.T, handler *mockDebuginfoHandler, uploadHandler http.Handler) DebugInfoClient {
	t.Helper()
	router := mux.NewRouter()
	debuginfov1alpha1connect.RegisterDebuginfoServiceHandler(router, handler)
	router.Handle("/debuginfo.v1alpha1.DebuginfoService/Upload/{gnu_build_id}", uploadHandler).Methods("POST")
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return &testDebugInfoClient{
		DebuginfoServiceClient: debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL),
		httpClient:             server.Client(),
		baseURL:                server.URL,
	}
}

func acceptUploadHandler(t *testing.T, resultCh chan<- uploadResult) (*mockDebuginfoHandler, http.Handler) {
	t.Helper()
	handler := &mockDebuginfoHandler{
		shouldInitiateFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
			return connect.NewResponse(&debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: true,
			}), nil
		},
		uploadFinishedFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error) {
			return connect.NewResponse(&debuginfov1alpha1.UploadFinishedResponse{}), nil
		},
	}
	uploadHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buildID := mux.Vars(r)["gnu_build_id"]
		data, _ := io.ReadAll(r.Body)
		resultCh <- uploadResult{
			buildID: buildID,
			data:    data,
		}
		w.WriteHeader(http.StatusOK)
	})
	return handler, uploadHTTP
}

func TestAttemptUpload_Success(t *testing.T) {
	fileData := []byte("hello debuginfo world")
	resultCh := make(chan uploadResult, 1)
	handler, uploadHTTP := acceptUploadHandler(t, resultCh)
	client := startMockServer(t, handler, uploadHTTP)
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(1, 2)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "abc123", newMockReadAtCloser(fileData))
	require.NoError(t, err)

	select {
	case res := <-resultCh:
		require.Equal(t, "abc123", res.buildID)
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to receive upload")
	}
	require.Equal(t, float64(len(fileData)), counterValue(counter))
}

func TestAttemptUpload_ServerDeclinesUpload(t *testing.T) {
	handler := &mockDebuginfoHandler{
		shouldInitiateFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
			return connect.NewResponse(&debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: false,
				Reason:               "already exists",
			}), nil
		},
	}
	uploadHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upload should not be called when server declines")
	})
	client := startMockServer(t, handler, uploadHTTP)
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(3, 4)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "def456", newMockReadAtCloser([]byte("data")))
	require.NoError(t, err)
	require.Equal(t, float64(0), counterValue(counter))

	_, cached := u.retry.Get(fileID)
	require.True(t, cached, "fileID should be in retry cache after declined upload")
}

func TestAttemptUpload_UploadInProgress(t *testing.T) {
	handler := &mockDebuginfoHandler{
		shouldInitiateFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
			return connect.NewResponse(&debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: false,
				Reason:               ReasonUploadInProgress,
			}), nil
		},
	}
	uploadHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upload should not be called")
	})
	client := startMockServer(t, handler, uploadHTTP)
	u, _ := newTestUploader(t)
	fileID := libpf.NewFileID(5, 6)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "ghi789", newMockReadAtCloser([]byte("data")))
	require.NoError(t, err)

	_, cached := u.retry.Get(fileID)
	require.True(t, cached, "fileID should be in retry cache for in-progress reason")
}

func TestAttemptUpload_EmptyBuildID(t *testing.T) {
	var receivedFile *debuginfov1alpha1.FileMetadata

	handler := &mockDebuginfoHandler{
		shouldInitiateFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
			receivedFile = req.Msg.File
			return connect.NewResponse(&debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: false,
			}), nil
		},
	}
	uploadHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upload should not be called")
	})
	client := startMockServer(t, handler, uploadHTTP)
	u, _ := newTestUploader(t)
	fileID := libpf.NewFileID(7, 8)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "", newMockReadAtCloser([]byte("data")))
	require.NoError(t, err)
	require.Equal(t, "", receivedFile.GetGnuBuildId())
	require.Equal(t, fileID.StringNoQuotes(), receivedFile.GetOtelFileId())
}

func TestAttemptUpload_LargeFile(t *testing.T) {
	dataSize := 6*1024*1024 + 512*1024 // ~6.5MB
	fileData := make([]byte, dataSize)
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	resultCh := make(chan uploadResult, 1)
	handler, uploadHTTP := acceptUploadHandler(t, resultCh)
	client := startMockServer(t, handler, uploadHTTP)
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(9, 10)

	err := u.attemptUpload(context.Background(), client, fileID, "big.so", "build1", newMockReadAtCloser(fileData))
	require.NoError(t, err)

	select {
	case res := <-resultCh:
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to receive upload")
	}
	require.Equal(t, float64(dataSize), counterValue(counter))
}

func TestUpload_Dedup(t *testing.T) {
	uploadCount := atomic.NewInt32(0)

	handler := &mockDebuginfoHandler{
		shouldInitiateFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.ShouldInitiateUploadRequest]) (*connect.Response[debuginfov1alpha1.ShouldInitiateUploadResponse], error) {
			uploadCount.Add(1)
			return connect.NewResponse(&debuginfov1alpha1.ShouldInitiateUploadResponse{
				ShouldInitiateUpload: true,
			}), nil
		},
		uploadFinishedFunc: func(ctx context.Context, req *connect.Request[debuginfov1alpha1.UploadFinishedRequest]) (*connect.Response[debuginfov1alpha1.UploadFinishedResponse], error) {
			return connect.NewResponse(&debuginfov1alpha1.UploadFinishedResponse{}), nil
		},
	}
	uploadHTTP := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	})
	client := startMockServer(t, handler, uploadHTTP)
	u, _ := newTestUploader(t)
	fileID := libpf.NewFileID(11, 12)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = u.Run(ctx)
	}()

	u.Upload(ctx, client, fileID, "test.so", "build1", newMockReadAtCloser([]byte("data")))
	u.Upload(ctx, client, fileID, "test.so", "build1", newMockReadAtCloser([]byte("data")))

	time.Sleep(500 * time.Millisecond)
	cancel()
	wg.Wait()

	require.Equal(t, int32(1), uploadCount.Load(), "expected exactly 1 upload, second should be deduped")
}

func TestUpload_WorkerProcessesQueue(t *testing.T) {
	fileData := []byte("worker-test-data")
	resultCh := make(chan uploadResult, 1)
	handler, uploadHTTP := acceptUploadHandler(t, resultCh)
	client := startMockServer(t, handler, uploadHTTP)
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(13, 14)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = u.Run(ctx)
	}()

	u.Upload(ctx, client, fileID, "worker.so", "build-worker", newMockReadAtCloser(fileData))

	select {
	case res := <-resultCh:
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for upload")
	}

	require.Eventually(t, func() bool {
		_, cached := u.retry.Get(fileID)
		return cached
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()

	require.Equal(t, float64(len(fileData)), counterValue(counter))
}

func TestMapShrink(t *testing.T) {
	tr := newInProgressTracker(0.2)
	r := rand.New(rand.NewSource(0))

	items := make([]libpf.FileID, 100)
	for i := 0; i < 100; i++ {
		items[i] = libpf.NewFileID(
			r.Uint64(),
			r.Uint64(),
		)

		tr.GetOrAdd(items[i])
	}

	if tr.maxSizeSeen != 100 {
		t.Errorf("expected 100, got %d", tr.maxSizeSeen)
	}

	for i := 0; i < 10; i++ {
		tr.Remove(items[i])
	}

	if tr.maxSizeSeen != 100 {
		t.Errorf("expected 100, got %d", tr.maxSizeSeen)
	}

	for i := 10; i < 20; i++ {
		tr.Remove(items[i])
	}

	if tr.maxSizeSeen != 83 {
		t.Errorf("expected 83, got %d", tr.maxSizeSeen)
	}

	for i := 10; i < 13; i++ {
		tr.GetOrAdd(items[i])
	}

	if tr.maxSizeSeen != 83 {
		t.Errorf("expected 83, got %d", tr.maxSizeSeen)
	}

	tr.GetOrAdd(items[13])

	if tr.maxSizeSeen != 84 {
		t.Errorf("expected 84, got %d", tr.maxSizeSeen)
	}
}
