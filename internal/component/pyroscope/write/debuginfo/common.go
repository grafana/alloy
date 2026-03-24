package debuginfo

import (
	"context"
	"sync"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

type Appender interface {
	// Upload dispatches the job recursively to each of the nested children, down to each write component,
	//down to Client and therefore parca's uploader
	Upload(j UploadJob)
	// Client returns the direct grpc client of the first nested child (down to write component)
	// this is a best-effort support for the proxy case - we do not support fan-out for proxy only one-to-one
	// forwarding to the first write endpoint.
	Client() debuginfogrpc.DebuginfoServiceClient
}

type Arguments struct {
	OnTargetSymbolizationEnabled bool   `alloy:"on_target_symbolization,attr,optional"`
	UploadEnabled                bool   `alloy:"upload,attr,optional"`
	CacheSize                    uint32 `alloy:"cache_size,attr,optional"`
	StripTextSection             bool   `alloy:"strip_text_section,attr,optional"`
	QueueSize                    uint32 `alloy:"queue_size,attr,optional"`
	WorkerNum                    int    `alloy:"worker_num,attr,optional"`
}

func NewClient(logger log.Logger, newClient func() (*grpc.ClientConn, error),
	metric prometheus.Counter, dataPath string) *Client {

	return &Client{
		newClient: newClient,
		metric:    metric,
		dataPath:  dataPath,

		logger:       logger,
		uploaderChan: make(chan *uploader, 1),
	}
}

// Client is per write-endpoint debug info upload client
// This structure serves two purposes:
//   - return the grpc client to the receive_http component
//   - perform the debug info upload from the current host by the ebpf profiler request
type Client struct {
	logger       log.Logger
	newClient    func() (*grpc.ClientConn, error)
	clientOnce   sync.Once
	cc           *grpc.ClientConn
	client       debuginfogrpc.DebuginfoServiceClient
	uploaderOnce sync.Once
	uploader     *uploader
	uploaderChan chan *uploader
	metric       prometheus.Counter
	dataPath     string
}

func (c *Client) Client() debuginfogrpc.DebuginfoServiceClient {
	c.clientOnce.Do(func() {
		var err error
		c.cc, err = c.newClient()
		if err != nil {
			_ = level.Error(c.logger).Log("msg", "error initializing debuginfo client", "err", err)
			return
		}
		c.client = debuginfogrpc.NewDebuginfoServiceClient(c.cc)
	})
	return c.client
}

func (c *Client) Upload(j UploadJob) {
	cc := c.Client()
	if cc == nil {
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

	c.uploader.upload(cc, j)
}

func (c *Client) Run(ctx context.Context) error {
	defer func() {
		if c.cc != nil {
			_ = c.cc.Close()
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case u := <-c.uploaderChan:
		return u.run(ctx)
	}
}
