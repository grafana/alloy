package debuginfo

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/pyroscope/api/gen/proto/go/debuginfo/v1alpha1/debuginfov1alpha1connect"
	"github.com/prometheus/client_golang/prometheus"
)

type Appender interface {
	// Upload dispatches the job recursively to each of the nested children, down to each write component,
	// down to Client and therefore the uploader.
	Upload(j UploadJob)
	// ConnectClient returns the Connect debuginfo client of the first nested child (down to write component).
	ConnectClient() debuginfov1alpha1connect.DebuginfoServiceClient
	// ConnectClients returns ALL Connect debuginfo clients from all nested children.
	// This is used by the receive_http proxy to fan-out uploads to all downstream endpoints.
	ConnectClients() []debuginfov1alpha1connect.DebuginfoServiceClient
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
	metric prometheus.Counter, dataPath string) *Client {

	return &Client{
		connectClient: connectClient,
		metric:        metric,
		dataPath:      dataPath,
		logger:        logger,
		uploaderChan:  make(chan *uploader, 1),
	}
}

// Client is per write-endpoint debug info upload client.
// This structure serves two purposes:
//   - return the connect client to the receive_http component for proxying
//   - perform the debug info upload from the current host by the ebpf profiler request
type Client struct {
	logger        log.Logger
	connectClient debuginfov1alpha1connect.DebuginfoServiceClient
	uploaderOnce  sync.Once
	uploader      *uploader
	uploaderChan  chan *uploader
	metric        prometheus.Counter
	dataPath      string
}

func (c *Client) ConnectClient() debuginfov1alpha1connect.DebuginfoServiceClient {
	return c.connectClient
}

func (c *Client) ConnectClients() []debuginfov1alpha1connect.DebuginfoServiceClient {
	if c.connectClient != nil {
		return []debuginfov1alpha1connect.DebuginfoServiceClient{c.connectClient}
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

	c.uploader.upload(c.connectClient, j)
}

func (c *Client) Run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case u := <-c.uploaderChan:
		return u.run(ctx)
	}
}
