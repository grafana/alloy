//go:build linux && (arm64 || amd64)

package reporter

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/parca/reporter/elfwriter"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfoclient"

	"connectrpc.com/connect"
	debuginfov1alpha1 "github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	lru "github.com/elastic/go-freelru"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup" //nolint:depguard

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
)

const (
	ReasonUploadInProgress = "A previous upload is still in-progress and not stale yet (only stale uploads can be retried)."
)

type uploadRequest struct {
	fileID   libpf.FileID
	fileName string
	buildID  string
	open     func() (process.ReadAtCloser, error)
	client   *debuginfoclient.Client
}

type PyroscopeSymbolUploader struct {
	logger log.Logger

	retry *lru.SyncedLRU[libpf.FileID, struct{}]

	stripTextSection bool
	tmp              string

	queue             chan uploadRequest
	inProgressTracker *inProgressTracker
	workerNum         int

	uploadRequestBytes prometheus.Counter
}

func NewPyroscopeSymbolUploader(
	logger log.Logger,
	cacheSize uint32,
	stripTextSection bool,
	queueSize uint32,
	workerNum int,
	cacheDir string,
	uploadRequestBytes prometheus.Counter,
) (*PyroscopeSymbolUploader, error) {

	retryCache, err := lru.NewSynced[libpf.FileID, struct{}](cacheSize, libpf.FileID.Hash32)
	if err != nil {
		return nil, err
	}

	cacheDirectory := filepath.Join(cacheDir, "symuploader")
	if _, err := os.Stat(cacheDirectory); os.IsNotExist(err) {
		level.Debug(logger).Log("msg", "creating cache directory", "path", cacheDirectory)
		if err := os.MkdirAll(cacheDirectory, os.ModePerm); err != nil {
			return nil, fmt.Errorf("failed to create cache directory (%s): %s", cacheDirectory, err)
		}
	}

	if err := filepath.Walk(cacheDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			level.Warn(logger).Log("msg", "failed to access cached file during walk", "path", path, "err", err)
			return nil
		}

		if info == nil {
			level.Warn(logger).Log("msg", "filepath.Walk returned nil FileInfo", "path", path)
			return nil
		}
		if info.IsDir() {
			return nil
		}

		if os.Remove(path) != nil {
			level.Warn(logger).Log("msg", "failed to remove cached file", "path", path)
		}

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to clean cache directory (%s): %s", cacheDirectory, err)
	}

	return &PyroscopeSymbolUploader{
		logger:             logger,
		retry:              retryCache,
		stripTextSection:   stripTextSection,
		tmp:                cacheDirectory,
		queue:              make(chan uploadRequest, queueSize),
		inProgressTracker:  newInProgressTracker(0.2),
		workerNum:          workerNum,
		uploadRequestBytes: uploadRequestBytes,
	}, nil
}

// inProgressTracker is a simple in-progress tracker that keeps track of which
// fileIDs are currently in-progress/enqueued to be uploaded.
type inProgressTracker struct {
	mu sync.Mutex
	m  map[libpf.FileID]struct{}

	// tracking metadata to know when to shrink the map as otherwise the map
	// may grow indefinitely.
	maxSizeSeen      int
	shrinkLimitRatio float64
}

// newInProgressTracker returns a new in-progress tracker that shrinks the
// tracking map when the maximum size seen is larger than the current size by
// the shrinkLimitRatio.
func newInProgressTracker(shrinkLimitRatio float64) *inProgressTracker {
	return &inProgressTracker{
		m:                make(map[libpf.FileID]struct{}),
		shrinkLimitRatio: shrinkLimitRatio,
	}
}

// GetOrAdd returns ensures that the fileID is in the in-progress state. If the
// fileID is already in the in-progress state it returns true.
func (i *inProgressTracker) GetOrAdd(fileID libpf.FileID) (alreadyInProgress bool) {
	i.mu.Lock()
	defer i.mu.Unlock()

	_, alreadyInProgress = i.m[fileID]
	i.m[fileID] = struct{}{}

	if len(i.m) > i.maxSizeSeen {
		i.maxSizeSeen = len(i.m)
	}

	return
}

// Remove removes the fileID from the in-progress state.
func (i *inProgressTracker) Remove(fileID libpf.FileID) {
	i.mu.Lock()
	defer i.mu.Unlock()

	delete(i.m, fileID)

	if i.shrinkLimitRatio > 0 &&
		int(float64(len(i.m))+float64(len(i.m))*i.shrinkLimitRatio) < i.maxSizeSeen {

		i.m = maps.Clone(i.m)
		i.maxSizeSeen = len(i.m)
	}
}

// Run starts the upload workers.
func (u *PyroscopeSymbolUploader) Run(ctx context.Context) error {
	var g errgroup.Group

	for i := 0; i < u.workerNum; i++ {
		g.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case req := <-u.queue:
					if err := u.attemptUpload(ctx, req.client, req.fileID, req.fileName, req.buildID, req.open); err != nil {
						level.Warn(u.logger).Log("msg", "failed to upload", "file_name", req.fileName, "build_id", req.buildID, "err", err)
					}
				}
			}
		})
	}

	return g.Wait()
}

// Upload enqueues a file for upload if it's not already in progress, or if it
// is marked not to be retried.
func (u *PyroscopeSymbolUploader) Upload(ctx context.Context, client *debuginfoclient.Client,
	fileID libpf.FileID, fileName string, buildID string,
	open func() (process.ReadAtCloser, error)) {

	// Skip virtual DSOs — they have no backing file and no build ID.
	if strings.HasPrefix(fileName, "linux-vdso") || strings.HasPrefix(fileName, "[vdso]") {
		return
	}

	_, ok := u.retry.Get(fileID)
	if ok {
		return
	}

	// Attempting to enqueue each fileID only once.
	alreadyInProgress := u.inProgressTracker.GetOrAdd(fileID)
	if alreadyInProgress {
		return
	}

	select {
	case <-ctx.Done():
		u.inProgressTracker.Remove(fileID)
	case u.queue <- uploadRequest{fileID: fileID, fileName: fileName, buildID: buildID, open: open, client: client}:
		// Nothing to do, we enqueued the request successfully.
	default:
		// The queue is full, we can't enqueue the request.
		u.inProgressTracker.Remove(fileID)
		level.Warn(u.logger).Log("msg", "failed to enqueue upload request, queue is full", "file_name", fileName, "build_id", buildID)
	}
}

func (u *PyroscopeSymbolUploader) attemptUpload(ctx context.Context, client *debuginfoclient.Client,
	fileID libpf.FileID, fileName string, buildID string,
	open func() (process.ReadAtCloser, error)) error {

	defer u.inProgressTracker.Remove(fileID)

	fileType := debuginfov1alpha1.FileMetadata_TYPE_EXECUTABLE_FULL
	if u.stripTextSection {
		fileType = debuginfov1alpha1.FileMetadata_TYPE_EXECUTABLE_NO_TEXT
	}

	// Step 1: ShouldInitiateUpload (unary RPC).
	resp, err := client.ShouldInitiateUpload(ctx, connect.NewRequest(&debuginfov1alpha1.ShouldInitiateUploadRequest{
		File: &debuginfov1alpha1.FileMetadata{
			GnuBuildId: buildID,
			OtelFileId: fileID.StringNoQuotes(),
			Name:       fileName,
			Type:       fileType,
		},
	}))
	if err != nil {
		return fmt.Errorf("ShouldInitiateUpload: %w", err)
	}

	l := log.With(u.logger,
		"file_name", fileName,
		"otel_file_id", fileID,
		"gnu_build_id", buildID,
	)

	level.Debug(l).Log("msg", "ShouldInitiateUpload result",
		"should_initiate_upload", resp.Msg.ShouldInitiateUpload,
		"reason", resp.Msg.Reason)

	if !resp.Msg.ShouldInitiateUpload {
		if resp.Msg.Reason == ReasonUploadInProgress {
			u.retry.AddWithLifetime(fileID, struct{}{}, 5*time.Minute)
			return nil
		}
		u.retry.Add(fileID, struct{}{})
		return nil
	}

	// Step 2: Prepare the file data.
	var r io.Reader
	var fileSize int64
	if !u.stripTextSection {
		f, err := open()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			if err.Error() == "no backing file for anonymous memory" {
				u.retry.Add(fileID, struct{}{})
				return nil
			}
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		size, err := readAtCloserSize(u.logger, f)
		if err != nil {
			return err
		}
		if size == 0 {
			u.retry.Add(fileID, struct{}{})
			return nil
		}

		fileSize = size
		r = io.NewSectionReader(f, 0, size)
	} else {
		f, err := os.Create(filepath.Join(u.tmp, fileID.StringNoQuotes()))
		if err != nil {
			return fmt.Errorf("create file: %w", err)
		}
		defer os.Remove(f.Name())
		defer f.Close()

		original, err := open()
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			if err.Error() == "no backing file for anonymous memory" {
				u.retry.Add(fileID, struct{}{})
				return nil
			}
			return fmt.Errorf("open original file: %w", err)
		}
		defer original.Close()

		if err := elfwriter.OnlyKeepDebug(f, original); err != nil {
			u.retry.Add(fileID, struct{}{})
			return fmt.Errorf("extract debuginfo: %w", err)
		}

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			u.retry.Add(fileID, struct{}{})
			return fmt.Errorf("seek extracted debuginfo to start: %w", err)
		}

		stat, err := f.Stat()
		if err != nil {
			u.retry.Add(fileID, struct{}{})
			return fmt.Errorf("stat file to upload: %w", err)
		}
		if stat.Size() == 0 {
			u.retry.Add(fileID, struct{}{})
			return nil
		}

		fileSize = stat.Size()
		r = f
	}

	// Step 3: HTTP POST upload.
	if err := client.Upload(ctx, buildID, r); err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	// Step 4: UploadFinished (unary RPC).
	if _, err := client.UploadFinished(ctx, connect.NewRequest(&debuginfov1alpha1.UploadFinishedRequest{
		GnuBuildId: buildID,
	})); err != nil {
		return fmt.Errorf("UploadFinished: %w", err)
	}

	u.uploadRequestBytes.Add(float64(fileSize))
	level.Debug(l).Log("msg", "upload succeeded", "bytes", fileSize)
	u.retry.Add(fileID, struct{}{})
	return nil
}

type Stater interface {
	Stat() (os.FileInfo, error)
}

// readAtCloserSize attempts to determine the size of the reader.
func readAtCloserSize(logger log.Logger, r process.ReadAtCloser) (int64, error) {
	stater, ok := r.(Stater)
	if !ok {
		level.Debug(logger).Log("msg", "ReadAtCloser is not a Stater, can't determine size")
		return 0, nil
	}

	stat, err := stater.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat file to upload: %w", err)
	}

	return stat.Size(), nil
}
