package queue

import (
	"context"
	"encoding/base64"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
	"github.com/prometheus/client_golang/prometheus"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"net/url"
	"path"
	"path/filepath"
	"time"
)

type remote struct {
	name     string
	b        *parquetwrite
	database *filequeue
	qm       *QueueManager
	wr       WriteClient
	writer   *fileQueueWriter
}

func newRemote(ed EndpointOptions, registerer prometheus.Registerer, l log.Logger, args Arguments, opts component.Options) (*remote, error) {
	wr, err := newWriteClient(ed)
	if err != nil {
		return nil, err
	}
	qm := NewQueueManager(
		newQueueManagerMetrics(registerer, ed.Name, ed.URL),
		l,
		newEWMARate(ewmaWeight, shardUpdateDuration),
		ed.QueueOptions,
		ed.MetadataOptions,
		labels.FromMap(args.ExternalLabels),
		wr,
		1*time.Minute,
		newPool(),
		&maxTimestamp{
			Gauge: prometheus.NewGauge(prometheus.GaugeOpts{
				Namespace: "prometheus",
				Subsystem: "remote_storage",
				Name:      "highest_timestamp_in_seconds",
				Help:      "Highest timestamp that has come into the remote storage via the Appender interface, in seconds since epoch.",
			}),
		},
		true,
		true,
	)
	q, err := newFileQueue(filepath.Join(opts.DataPath, "wal", wr.Name()), path.Join(opts.ID, wr.Name()))
	if err != nil {
		return nil, err
	}

	write := newFileQueueWriter(wr.Name(), qm, q, l, args.TTL, registerer)
	pw := newParquetWrite(q, args.BatchSize, args.FlushTime, l)
	return &remote{
		name:     wr.Name(),
		b:        pw,
		database: q,
		qm:       qm,
		wr:       wr,
		writer:   write,
	}, nil
}

func (r *remote) start(ctx context.Context) {
	started := make(chan struct{})
	go r.qm.Start(started)
	<-started
	go r.writer.Start(ctx)
}

func (r *remote) stop() {
	r.qm.Stop()
}

func newWriteClient(ed EndpointOptions) (WriteClient, error) {
	endUrl, err := url.Parse(ed.URL)
	if err != nil {
		return nil, err
	}
	cfgURL := &config_util.URL{URL: endUrl}
	if err != nil {
		return nil, err
	}
	name := ed.Name
	if len(ed.Name) == 0 {
		enc := base64.Encoding{}
		name = enc.EncodeToString([]byte(ed.URL))
	}

	wr, err := NewWriteClient(name, &ClientConfig{
		URL:              cfgURL,
		Timeout:          model.Duration(ed.RemoteTimeout),
		HTTPClientConfig: *ed.HTTPClientConfig.Convert(),
		SigV4Config:      nil,
		Headers:          ed.Headers,
		RetryOnRateLimit: ed.QueueOptions.RetryOnHTTP429,
	})

	return wr, err
}
