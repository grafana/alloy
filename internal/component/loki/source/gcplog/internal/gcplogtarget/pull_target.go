package gcplogtarget

// This code is copied from Promtail. The gcplogtarget package is used to
// configure and run the targets that can read log entries from cloud resource
// logs like bucket logs, load balancer logs, and Kubernetes cluster logs
// from GCP.

import (
	"context"
	"io"
	"sync"
	"time"

	//nolint:staticcheck // TODO: upgrade to v2
	"cloud.google.com/go/pubsub"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"google.golang.org/api/option"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/loki/source/gcplog/gcptypes"
)

// PullTarget represents a target that scrapes logs from a GCP project id and
// subscription and converts them to Loki log entries.
type PullTarget struct {
	metrics       *Metrics
	logger        log.Logger
	handler       loki.EntryHandler
	config        *gcptypes.PullConfig
	relabelConfig []*relabel.Config
	jobName       string

	// lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	backoff *backoff.Backoff

	// pubsub
	ps   io.Closer
	sub  pubsubSubscription
	msgs chan *pubsub.Message
}

// TODO(@tpaschalis) Expose this as Alloy configuration in the future.
var defaultBackoff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 10 * time.Second,
	MaxRetries: 0, // Retry forever
}

// pubsubSubscription allows us to mock pubsub for testing
type pubsubSubscription interface {
	Receive(ctx context.Context, f func(context.Context, *pubsub.Message)) error
}

// NewPullTarget returns the new instance of PullTarget.
func NewPullTarget(metrics *Metrics, logger log.Logger, handler loki.EntryHandler, jobName string, config *gcptypes.PullConfig, relabel []*relabel.Config, clientOptions ...option.ClientOption) (*PullTarget, error) {
	ctx, cancel := context.WithCancel(context.Background())
	ps, err := pubsub.NewClient(ctx, config.ProjectID, clientOptions...)
	if err != nil {
		cancel()
		return nil, err
	}

	target := &PullTarget{
		metrics:       metrics,
		logger:        logger,
		handler:       handler,
		relabelConfig: relabel,
		config:        config,
		jobName:       jobName,
		ctx:           ctx,
		cancel:        cancel,
		ps:            ps,
		sub:           ps.SubscriptionInProject(config.Subscription, config.ProjectID),
		backoff:       backoff.New(ctx, defaultBackoff),
		msgs:          make(chan *pubsub.Message),
	}

	go func() {
		err := target.run()
		if err != nil {
			_ = level.Error(logger).Log("msg", "loki.source.gcplog pull target shutdown with error", "err", err)
		}
	}()

	return target, nil
}

func (t *PullTarget) run() error {
	t.wg.Add(1)
	defer t.wg.Done()

	go t.consumeSubscription()

	lbls := make(model.LabelSet, len(t.config.Labels))
	for k, v := range t.config.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	for {
		select {
		case <-t.ctx.Done():
			return t.ctx.Err()
		case m := <-t.msgs:
			entry, err := parseGCPLogsEntry(m.Data, lbls, labels.EmptyLabels(), t.config.UseIncomingTimestamp, t.config.UseFullLine, t.relabelConfig)
			if err != nil {
				level.Error(t.logger).Log("event", "error formating log entry", "cause", err)
				m.Ack()
				break
			}
			t.handler.Chan() <- entry
			m.Ack() // Ack only after log is sent.
			t.metrics.gcplogEntries.WithLabelValues(t.config.ProjectID).Inc()
		}
	}
}

func (t *PullTarget) consumeSubscription() {
	// NOTE(kavi): `cancel` the context as exiting from this goroutine should stop main `run` loop
	// It makesense as no more messages will be received.
	defer t.cancel()

	for t.backoff.Ongoing() {
		err := t.sub.Receive(t.ctx, func(ctx context.Context, m *pubsub.Message) {
			t.msgs <- m
			t.backoff.Reset()
		})
		if err != nil {
			level.Error(t.logger).Log("msg", "failed to receive pubsub messages", "error", err)
			t.metrics.gcplogErrors.WithLabelValues(t.config.ProjectID).Inc()
			t.metrics.gcplogTargetLastSuccessScrape.WithLabelValues(t.config.ProjectID, t.config.Subscription).SetToCurrentTime()
			t.backoff.Wait()
		}
	}
}

// Labels return the model.LabelSet that the target applies to log entries.
func (t *PullTarget) Labels() model.LabelSet {
	lbls := make(model.LabelSet, len(t.config.Labels))
	for k, v := range t.config.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}
	return lbls
}

// Details returns some debug information about the target.
func (t *PullTarget) Details() map[string]string {
	return map[string]string{
		"strategy": "pull",
		"labels":   t.Labels().String(),
	}
}

// Stop shuts the target down.
func (t *PullTarget) Stop() error {
	t.cancel()
	t.wg.Wait()
	t.handler.Stop()
	t.ps.Close()
	return nil
}
