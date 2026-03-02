//go:build linux && (arm64 || amd64)

package reporter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/parca/reporter/elfwriter"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	debuginfopb "buf.build/gen/go/parca-dev/parca/protocolbuffers/go/parca/debuginfo/v1alpha1"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	lru "github.com/elastic/go-freelru"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup" //nolint:depguard
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/process"
)

type uploadRequest struct {
	fileID   libpf.FileID
	fileName string
	buildID  string
	open     func() (process.ReadAtCloser, error)
	client   debuginfogrpc.DebuginfoServiceClient
}

type ParcaSymbolUploader struct {
	logger log.Logger

	httpClient *http.Client

	retry *lru.SyncedLRU[libpf.FileID, struct{}]

	stripTextSection bool
	tmp              string

	queue             chan uploadRequest
	inProgressTracker *inProgressTracker
	workerNum         int

	uploadRequestBytes prometheus.Counter
}

func NewParcaSymbolUploader(
	logger log.Logger,
	cacheSize uint32,
	stripTextSection bool,
	queueSize uint32,
	workerNum int,
	cacheDir string,
	uploadRequestBytes prometheus.Counter,
) (*ParcaSymbolUploader, error) {

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

	return &ParcaSymbolUploader{
		logger:             logger,
		httpClient:         http.DefaultClient,
		retry:              retryCache,
		stripTextSection:   stripTextSection,
		tmp:                cacheDirectory,
		queue:              make(chan uploadRequest, queueSize),
		inProgressTracker:  newInProgressTracker(0.2),
		workerNum:          workerNum,
		uploadRequestBytes: uploadRequestBytes,
	}, nil
}

const (
	ReasonUploadInProgress = "A previous upload is still in-progress and not stale yet (only stale uploads can be retried)."
)

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

// Start starts the upload workers.
func (u *ParcaSymbolUploader) Run(ctx context.Context) error {
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
func (u *ParcaSymbolUploader) Upload(ctx context.Context, client debuginfogrpc.DebuginfoServiceClient,
	fileID libpf.FileID, fileName string, buildID string,
	open func() (process.ReadAtCloser, error)) {

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

// attemptUpload attempts to upload the file with the given fileID and buildID.
func (u *ParcaSymbolUploader) attemptUpload(ctx context.Context, client debuginfogrpc.DebuginfoServiceClient, fileID libpf.FileID, fileName string, buildID string,
	open func() (process.ReadAtCloser, error)) error {

	defer u.inProgressTracker.Remove(fileID)

	buildIDType := debuginfopb.BuildIDType_BUILD_ID_TYPE_GNU
	if buildID == "" {
		buildIDType = debuginfopb.BuildIDType_BUILD_ID_TYPE_HASH
		buildID = fileID.StringNoQuotes()
	}

	shouldInitiateUploadResp, err := client.ShouldInitiateUpload(ctx, &debuginfopb.ShouldInitiateUploadRequest{
		BuildId:     buildID,
		BuildIdType: buildIDType,
		Type:        debuginfopb.DebuginfoType_DEBUGINFO_TYPE_DEBUGINFO_UNSPECIFIED,
	})
	if err != nil {
		return err
	}

	l := log.With(u.logger,
		"file_name", fileName,
		"file_id", fileID,
		"build_id", buildID,
	)

	level.Debug(l).Log("msg", "ShouldInitiateUpload result",
		"should_initiate_upload", shouldInitiateUploadResp.ShouldInitiateUpload,
		"reason", shouldInitiateUploadResp.Reason)

	if !shouldInitiateUploadResp.ShouldInitiateUpload {
		// This can happen when two agents simultaneously try to upload the
		// same file. The other agent already started the upload so we don't
		// need to do it again, however the upload may fail so we should retry
		// after a while.
		if shouldInitiateUploadResp.Reason == ReasonUploadInProgress {
			u.retry.AddWithLifetime(fileID, struct{}{}, 5*time.Minute)
			return nil
		}
		u.retry.Add(fileID, struct{}{})
		return nil
	}

	var (
		r    io.Reader
		size int64
	)
	if !u.stripTextSection {
		// We're not stripping the text section so we can upload the original file.
		f, err := open()
		if err != nil {
			if os.IsNotExist(err) {
				// File doesn't exist, likely because the process is already
				// gone.
				return nil
			}
			if err.Error() == "no backing file for anonymous memory" {
				// This is an anonymous memory mapping, it's not backed by
				// a file so we will never be able to extract debuginfo.
				u.retry.Add(fileID, struct{}{})
				return nil
			}
			return fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		size, err = readAtCloserSize(u.logger, f)
		if err != nil {
			return err
		}
		if size == 0 {
			// The original file is empty no need to ever upload it.
			u.retry.Add(fileID, struct{}{})
			return nil
		}

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
			os.Remove(f.Name())
			if os.IsNotExist(err) {
				// Original file doesn't exist the process is likely
				// already gone.
				return nil
			}
			if err.Error() == "no backing file for anonymous memory" {
				// This is an anonymous memory mapping, it's not backed by
				// a file so we will never be able to extract debuginfo.
				u.retry.Add(fileID, struct{}{})
				return nil
			}
			return fmt.Errorf("open original file: %w", err)
		}
		defer original.Close()

		if err := elfwriter.OnlyKeepDebug(f, original); err != nil {
			os.Remove(f.Name())
			// If we can't extract the debuginfo we can't upload the file.
			u.retry.Add(fileID, struct{}{})
			return fmt.Errorf("extract debuginfo: %w", err)
		}

		if _, err := f.Seek(0, io.SeekStart); err != nil {
			os.Remove(f.Name())
			// Something is probably seriously wrong so don't retry.
			u.retry.Add(fileID, struct{}{})
			return fmt.Errorf("seek extracted debuginfo to start: %w", err)
		}

		stat, err := f.Stat()
		if err != nil {
			os.Remove(f.Name())
			// Something is probably seriously wrong so don't retry.
			u.retry.Add(fileID, struct{}{})
			return fmt.Errorf("stat file to upload: %w", err)
		}
		size = stat.Size()

		if size == 0 {
			os.Remove(f.Name())
			// Extraction is a deterministic process so if the file is empty we
			// will never be able to extract non-zero debuginfo the original
			// binary.
			u.retry.Add(fileID, struct{}{})
			return nil
		}

		r = f
	}
	initiateUploadResp, err := client.InitiateUpload(ctx, &debuginfopb.InitiateUploadRequest{
		BuildId:     buildID,
		BuildIdType: buildIDType,
		Type:        debuginfopb.DebuginfoType_DEBUGINFO_TYPE_DEBUGINFO_UNSPECIFIED,
		Hash:        fileID.StringNoQuotes(),
		Size:        size,
	})

	if err != nil {
		level.Debug(u.logger).Log("msg", "InitiateUpload", "err", err)
		if status.Code(err) == codes.FailedPrecondition {
			// This is a race that can happen when multiple agents are trying
			// to upload the same file. This happens when another upload is
			// still in progress. Since we don't know if it will succeed or not
			// we retry after a while.
			u.retry.AddWithLifetime(fileID, struct{}{}, 5*time.Minute)
			return nil
		}
		if status.Code(err) == codes.AlreadyExists {
			// This is a race that can happen when multiple agents are trying
			// to upload the same file. The other upload already succeeded so
			// we don't need to upload it again.
			u.retry.Add(fileID, struct{}{})
			return nil
		}
		if status.Code(err) == codes.InvalidArgument {
			// This will never succeed, no need to retry.
			u.retry.Add(fileID, struct{}{})
			return nil
		}
		return err
	}
	level.Debug(u.logger).Log("msg", "InitiateUpload", "res", fmt.Sprintf("%+v", initiateUploadResp))

	if initiateUploadResp.UploadInstructions == nil {
		u.retry.Add(fileID, struct{}{})
		return nil
	}

	instructions := initiateUploadResp.UploadInstructions
	var uploadedBytes uint64
	switch instructions.UploadStrategy {
	case debuginfopb.UploadInstructions_UPLOAD_STRATEGY_SIGNED_URL:
		if err := u.uploadViaSignedURL(ctx, instructions.SignedUrl, r, size); err != nil {
			return err
		}
		uploadedBytes = uint64(size)
	case debuginfopb.UploadInstructions_UPLOAD_STRATEGY_GRPC:
		var err error
		uploadedBytes, err = NewGrpcUploadClient(client).Upload(ctx, instructions, r)
		if err != nil {
			return err
		}
	default:
		// No clue what to do with this upload strategy.
		level.Warn(u.logger).Log("msg", "unknown upload strategy", "strategy", instructions.UploadStrategy)
		u.retry.Add(fileID, struct{}{})
		return nil
	}

	u.uploadRequestBytes.Add(float64(uploadedBytes))

	_, err = client.MarkUploadFinished(ctx, &debuginfopb.MarkUploadFinishedRequest{
		BuildId:  buildID,
		UploadId: initiateUploadResp.UploadInstructions.UploadId,
	})
	if err != nil {
		level.Debug(u.logger).Log("msg", "upload failed", "file_name", fileName, "build_id", buildID, "err", err)
		return err
	}

	level.Debug(u.logger).Log("msg", "upload succeeded", "file_name", fileName, "build_id", buildID, "bytes", uploadedBytes)
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

// uploadViaSignedURL uploads the reader to the signed URL.
func (u *ParcaSymbolUploader) uploadViaSignedURL(ctx context.Context, url string, r io.Reader, size int64) error {
	// Client is closing the reader if the reader is also closer.
	// We need to wrap the reader to avoid this.
	// We want to have total control over the reader.
	r = bufio.NewReader(r)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, r)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.ContentLength = size
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do upload request: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status code: %d, msg: %s", resp.StatusCode, string(data))
	}

	return nil
}
