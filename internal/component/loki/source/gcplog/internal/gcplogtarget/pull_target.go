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
	"cloud.google.com/go/pubsub/v2"
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
	recv          loki.LogsReceiver
	config        *gcptypes.PullConfig
	relabelConfig []*relabel.Config

	// lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	backoff *backoff.Backoff

	// pubsub
	client     io.Closer
	subscriber subscriber
}

// TODO(@tpaschalis) Expose this as Alloy configuration in the future.
var defaultBackoff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 10 * time.Second,
	MaxRetries: 0, // Retry forever
}

// subscriber allows us to mock pubsub for testing
type subscriber interface {
	Receive(ctx context.Context, f func(context.Context, *pubsub.Message)) error
}

// NewPullTarget returns the new instance of PullTarget.
func NewPullTarget(
	metrics *Metrics,
	logger log.Logger,
	recv loki.LogsReceiver,
	config *gcptypes.PullConfig,
	relabel []*relabel.Config,
	clientOptions ...option.ClientOption,
) (*PullTarget, error) {

	ctx, cancel := context.WithCancel(context.Background())
	client, err := pubsub.NewClient(ctx, config.ProjectID, clientOptions...)
	if err != nil {
		cancel()
		return nil, err
	}

	subscriber := client.Subscriber(config.Subscription)
	subscriber.ReceiveSettings.MaxOutstandingBytes = int(config.Limit.MaxOutstandingBytes)
	subscriber.ReceiveSettings.MaxOutstandingMessages = config.Limit.MaxOutstandingMessages

	return &PullTarget{
		metrics:       metrics,
		logger:        logger,
		recv:          recv,
		relabelConfig: relabel,
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		client:        client,
		subscriber:    subscriber,
		backoff:       backoff.New(ctx, defaultBackoff),
	}, nil
}

func (t *PullTarget) Run() error {
	t.wg.Go(func() {
		lbls := t.labels()

		for t.backoff.Ongoing() {
			err := t.subscriber.Receive(t.ctx, func(ctx context.Context, m *pubsub.Message) {
				entry, err := parseLogEntry(m.Data, labels.NewBuilder(labels.EmptyLabels()), t.relabelConfig, parseOptions{
					fixedLabels:          lbls,
					useFullLine:          t.config.UseFullLine,
					useIncomingTimestamp: t.config.UseIncomingTimestamp,
				})
				if err != nil {
					level.Error(t.logger).Log("msg", "could not parse log entry", "error", err)
					t.metrics.gcplogErrors.WithLabelValues(t.config.ProjectID).Inc()
					// NOTE: We want to call Ack here since we cannot process the message.
					m.Ack()
					return
				}

				select {
				case t.recv.Chan() <- entry:
					m.Ack()
					t.metrics.gcplogEntries.WithLabelValues(t.config.ProjectID).Inc()
				case <-ctx.Done():
					m.Nack()
				}

				t.backoff.Reset()
			})

			if err != nil {
				level.Error(t.logger).Log("msg", "failed to receive pubsub messages", "error", err)
				t.metrics.gcplogErrors.WithLabelValues(t.config.ProjectID).Inc()
				t.metrics.gcplogTargetLastSuccessScrape.WithLabelValues(t.config.ProjectID, t.config.Subscription).SetToCurrentTime()
				t.backoff.Wait()
			}
		}
	})
	return nil
}

func (t *PullTarget) Stop() {
	t.cancel()
	t.wg.Wait()
	t.client.Close()
}

func (t *PullTarget) labels() model.LabelSet {
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
		"labels":   t.labels().String(),
	}
}
