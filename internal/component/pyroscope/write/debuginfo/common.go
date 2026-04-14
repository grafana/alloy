package debuginfo

import (
	"context"
	"io"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/client_golang/prometheus"
)

type Client interface {
	debuginfov1alpha1connect.DebuginfoServiceClient
	Upload(ctx context.Context, buildID string, body io.Reader) error
}

type Appender interface {
	Upload(j UploadJob)
	DebugInfoClients() []Client
}

type Arguments struct {
	OnTargetSymbolizationEnabled bool   `alloy:"on_target_symbolization,attr,optional"`
	UploadEnabled                bool   `alloy:"upload,attr,optional"`
	CacheSize                    uint32 `alloy:"cache_size,attr,optional"`
	StripTextSection             bool   `alloy:"strip_text_section,attr,optional"`
	QueueSize                    uint32 `alloy:"queue_size,attr,optional"`
	WorkerNum                    int    `alloy:"worker_num,attr,optional"`
}

func NewUploader(logger log.Logger, client Client,
	metric prometheus.Counter, dataPath string) *Uploader {

	return &Uploader{
		client:       client,
		metric:       metric,
		dataPath:     dataPath,
		logger:       logger,
		uploaderChan: make(chan *uploader, 1),
	}
}

type Uploader struct {
	logger       log.Logger
	client       Client
	uploaderOnce sync.Once
	uploader     *uploader
	uploaderChan chan *uploader
	metric       prometheus.Counter
	dataPath     string
}

func (c *Uploader) DebugInfoClients() []Client {
	if c.client != nil {
		return []Client{c.client}
	}
	return nil
}

func (c *Uploader) Upload(j UploadJob) {
	if c.client == nil {
		return
	}
	c.uploaderOnce.Do(func() {
		var err error
		c.uploader, err = c.newUploader(j)
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "error initializing debuginfo uploader", "err", err)
		} else {
			c.uploaderChan <- c.uploader
		}
	})
	if c.uploader == nil {
		_ = level.Error(c.logger).Log("msg", "debuginfo uploader not initialized")
		return
	}

	c.uploader.upload(c.client, j)
}

func (c *Uploader) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case u := <-c.uploaderChan:
		return u.run(ctx)
	}
}
