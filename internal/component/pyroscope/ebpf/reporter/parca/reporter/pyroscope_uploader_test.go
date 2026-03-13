//go:build linux && (arm64 || amd64)

package reporter

import (
	"bytes"
	"context"
	"math/rand"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/go-kit/log"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
	"go.uber.org/atomic"
)

// mockDebuginfoHandler implements debuginfov1alpha1connect.DebuginfoServiceHandler.
type mockDebuginfoHandler struct {
	uploadFunc func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error
}

func (m *mockDebuginfoHandler) Upload(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
	return m.uploadFunc(ctx, stream)
}

// mockReadAtCloser wraps bytes.Reader to implement process.ReadAtCloser + Stater.
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

func startMockServer(t *testing.T, handler *mockDebuginfoHandler) debuginfov1alpha1connect.DebuginfoServiceClient {
	t.Helper()
	_, h := debuginfov1alpha1connect.NewDebuginfoServiceHandler(handler)
	// Bidi streaming requires HTTP/2, so use TLS test server.
	server := httptest.NewUnstartedServer(h)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	return debuginfov1alpha1connect.NewDebuginfoServiceClient(server.Client(), server.URL)
}

func counterValue(c prometheus.Counter) float64 {
	m := &dto.Metric{}
	_ = c.(prometheus.Metric).Write(m)
	return m.GetCounter().GetValue()
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

	// adding up to 83 doesn't change anything
	for i := 10; i < 13; i++ {
		tr.GetOrAdd(items[i])
	}

	if tr.maxSizeSeen != 83 {
		t.Errorf("expected 83, got %d", tr.maxSizeSeen)
	}

	// adding 84th item should increases the max size
	tr.GetOrAdd(items[13])

	if tr.maxSizeSeen != 84 {
		t.Errorf("expected 84, got %d", tr.maxSizeSeen)
	}
}

// receiveAllChunks drains chunk messages from a bidi stream, returning the concatenated data.
func receiveAllChunks(stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) ([]byte, int) {
	var data []byte
	chunks := 0
	for {
		req, err := stream.Receive()
		if err != nil {
			break
		}
		if chunk := req.GetChunk(); chunk != nil {
			chunks++
			data = append(data, chunk.GetChunk()...)
		}
	}
	return data, chunks
}

type uploadResult struct {
	buildID  string
	fileName string
	fileType debuginfov1alpha1.FileMetadata_Type
	data     []byte
	chunks   int
}

func acceptUploadHandler(t *testing.T, resultCh chan<- uploadResult) *mockDebuginfoHandler {
	t.Helper()
	return &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			req, err := stream.Receive()
			if err != nil {
				return err
			}
			init := req.GetInit()

			if err := stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: true,
					},
				},
			}); err != nil {
				return err
			}

			data, chunks := receiveAllChunks(stream)
			resultCh <- uploadResult{
				buildID:  init.GetFile().GetGnuBuildId(),
				fileName: init.GetFile().GetName(),
				fileType: init.GetFile().GetType(),
				data:     data,
				chunks:   chunks,
			}
			return nil
		},
	}
}

func TestAttemptUpload_Success(t *testing.T) {
	fileData := []byte("hello debuginfo world")
	resultCh := make(chan uploadResult, 1)
	client := startMockServer(t, acceptUploadHandler(t, resultCh))
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(1, 2)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "abc123", newMockReadAtCloser(fileData))
	require.NoError(t, err)

	select {
	case res := <-resultCh:
		require.Equal(t, "abc123", res.buildID)
		require.Equal(t, "test.so", res.fileName)
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to receive upload")
	}
	require.Equal(t, float64(len(fileData)), counterValue(counter))
}

func TestAttemptUpload_ServerDeclinesUpload(t *testing.T) {
	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			if _, err := stream.Receive(); err != nil {
				return err
			}
			return stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: false,
						Reason:               "already exists",
					},
				},
			})
		},
	}

	client := startMockServer(t, handler)
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(3, 4)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "def456", newMockReadAtCloser([]byte("data")))
	require.NoError(t, err)
	require.Equal(t, float64(0), counterValue(counter))

	// Verify the fileID was cached — a second Upload call should be skipped.
	_, cached := u.retry.Get(fileID)
	require.True(t, cached, "fileID should be in retry cache after declined upload")
}

func TestAttemptUpload_UploadInProgress(t *testing.T) {
	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			if _, err := stream.Receive(); err != nil {
				return err
			}
			return stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: false,
						Reason:               ReasonUploadInProgress,
					},
				},
			})
		},
	}

	client := startMockServer(t, handler)
	u, _ := newTestUploader(t)
	fileID := libpf.NewFileID(5, 6)

	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "ghi789", newMockReadAtCloser([]byte("data")))
	require.NoError(t, err)

	// Should be cached with limited lifetime (not permanent).
	_, cached := u.retry.Get(fileID)
	require.True(t, cached, "fileID should be in retry cache for in-progress reason")
}

func TestAttemptUpload_EmptyBuildID(t *testing.T) {
	var receivedFile *debuginfov1alpha1.FileMetadata

	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			req, err := stream.Receive()
			if err != nil {
				return err
			}
			receivedFile = req.GetInit().GetFile()
			return stream.Send(&debuginfov1alpha1.UploadResponse{
				Data: &debuginfov1alpha1.UploadResponse_Init{
					Init: &debuginfov1alpha1.ShouldInitiateUploadResponse{
						ShouldInitiateUpload: false,
					},
				},
			})
		},
	}

	client := startMockServer(t, handler)
	u, _ := newTestUploader(t)
	fileID := libpf.NewFileID(7, 8)

	// Pass empty buildID — GnuBuildId should be empty, OtelFileId should have the fileID.
	err := u.attemptUpload(context.Background(), client, fileID, "test.so", "", newMockReadAtCloser([]byte("data")))
	require.NoError(t, err)
	require.Equal(t, "", receivedFile.GetGnuBuildId())
	require.Equal(t, fileID.StringNoQuotes(), receivedFile.GetOtelFileId())
}

func TestAttemptUpload_LargeFile_MultipleChunks(t *testing.T) {
	// Create data larger than ChunkSize (3MB) to force multiple chunks.
	dataSize := ChunkSize*2 + 1024*512 // ~6.5MB → 3 chunks
	fileData := make([]byte, dataSize)
	for i := range fileData {
		fileData[i] = byte(i % 256)
	}

	resultCh := make(chan uploadResult, 1)
	client := startMockServer(t, acceptUploadHandler(t, resultCh))
	u, counter := newTestUploader(t)
	fileID := libpf.NewFileID(9, 10)

	err := u.attemptUpload(context.Background(), client, fileID, "big.so", "build1", newMockReadAtCloser(fileData))
	require.NoError(t, err)

	select {
	case res := <-resultCh:
		require.True(t, res.chunks >= 3, "expected at least 3 chunks, got %d", res.chunks)
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to receive upload")
	}
	require.Equal(t, float64(dataSize), counterValue(counter))
}

func TestUpload_Dedup(t *testing.T) {
	uploadCount := atomic.NewInt32(0)

	handler := &mockDebuginfoHandler{
		uploadFunc: func(ctx context.Context, stream *connect.BidiStream[debuginfov1alpha1.UploadRequest, debuginfov1alpha1.UploadResponse]) error {
			uploadCount.Add(1)
			if _, err := stream.Receive(); err != nil {
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
			// Drain chunks.
			for {
				_, err := stream.Receive()
				if err != nil {
					break
				}
			}
			return nil
		},
	}

	client := startMockServer(t, handler)
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

	// Enqueue same fileID twice quickly.
	u.Upload(ctx, client, fileID, "test.so", "build1", newMockReadAtCloser([]byte("data")))
	u.Upload(ctx, client, fileID, "test.so", "build1", newMockReadAtCloser([]byte("data")))

	// Wait for worker to process.
	time.Sleep(500 * time.Millisecond)
	cancel()
	wg.Wait()

	require.Equal(t, int32(1), uploadCount.Load(), "expected exactly 1 upload, second should be deduped")
}

func TestUpload_WorkerProcessesQueue(t *testing.T) {
	fileData := []byte("worker-test-data")
	resultCh := make(chan uploadResult, 1)
	client := startMockServer(t, acceptUploadHandler(t, resultCh))
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

	// Enqueue via Upload (goes through queue → worker → attemptUpload).
	u.Upload(ctx, client, fileID, "worker.so", "build-worker", newMockReadAtCloser(fileData))

	// Wait for the upload to complete on the server side.
	select {
	case res := <-resultCh:
		require.Equal(t, fileData, res.data)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for upload")
	}

	cancel()
	wg.Wait()

	require.Equal(t, float64(len(fileData)), counterValue(counter))
}
