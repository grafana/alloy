package debuginfo

import (
	"context"
	"net/http"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component/pyroscope/ebpf/reporter/parca/reporter"
)

type Endpoint = reporter.Endpoint

type Appender interface {
	Upload(j UploadJob)
	DebugInfoEndpoints() []Endpoint
}

type Arguments struct {
	OnTargetSymbolizationEnabled bool   `alloy:"on_target_symbolization,attr,optional"`
	UploadEnabled                bool   `alloy:"upload,attr,optional"`
	CacheSize                    uint32 `alloy:"cache_size,attr,optional"`
	StripTextSection             bool   `alloy:"strip_text_section,attr,optional"`
	QueueSize                    uint32 `alloy:"queue_size,attr,optional"`
	WorkerNum                    int    `alloy:"worker_num,attr,optional"`
}

func NewClient(logger log.Logger, connectClient debuginfov1alpha1connect.DebuginfoServiceClient,
	httpClient *http.Client, baseURL string,
	metric prometheus.Counter, dataPath string) *Client {

	return &Client{
		connectClient: connectClient,
		httpClient:    httpClient,
		baseURL:       baseURL,
		metric:        metric,
		dataPath:      dataPath,
		logger:        logger,
		uploaderChan:  make(chan *uploader, 1),
	}
}

type Client struct {
	logger        log.Logger
	connectClient debuginfov1alpha1connect.DebuginfoServiceClient
	httpClient    *http.Client
	baseURL       string
	uploaderOnce  sync.Once
	uploader      *uploader
	uploaderChan  chan *uploader
	metric        prometheus.Counter
	dataPath      string
}

func (c *Client) DebugInfoEndpoints() []Endpoint {
	if c.connectClient != nil {
		return []Endpoint{{
			ConnectClient: c.connectClient,
			HTTPClient:    c.httpClient,
			BaseURL:       c.baseURL,
		}}
	}
	return nil
}

func (c *Client) Upload(j UploadJob) {
	if c.connectClient == nil {
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

	c.uploader.upload(Endpoint{
		ConnectClient: c.connectClient,
		HTTPClient:    c.httpClient,
		BaseURL:       c.baseURL,
	}, j)
}

func (c *Client) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case u := <-c.uploaderChan:
		return u.run(ctx)
	}
}
