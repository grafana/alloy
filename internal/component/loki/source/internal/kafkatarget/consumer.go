package kafkatarget

// This code is copied from Promtail (https://github.com/grafana/loki/commit/065bee7e72b00d800431f4b70f0d673d6e0e7a2b). The kafkatarget package is used to
// configure and run the targets that can read kafka entries and forward them
// to other loki components.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

var defaultBackOff = backoff.Config{
	MinBackoff: 1 * time.Second,
	MaxBackoff: 60 * time.Second,
	MaxRetries: 20,
}

type Target interface {
	// DiscoveredLabels returns labels discovered before any relabeling.
	DiscoveredLabels() model.LabelSet
	// Labels returns labels that are added to this target and its stream.
	Labels() model.LabelSet
	// Details is additional information about this target specific to its type
	Details() any
}

type RunnableTarget interface {
	run()
	Target
}

type TargetDiscoverer interface {
	NewTarget(sarama.ConsumerGroupSession, sarama.ConsumerGroupClaim) (RunnableTarget, error)
}

// consumer handle a group consumer instance.
// It will create a new target for every consumer claim using the `TargetDiscoverer`.
type consumer struct {
	sarama.ConsumerGroup
	discoverer TargetDiscoverer
	logger     log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mutex          sync.Mutex // used during rebalancing setup and tear down
	activeTargets  []Target
	droppedTargets []Target
}

// start starts the consumer for a given list of topics.
func (c *consumer) start(ctx context.Context, topics []string) {
	c.wg.Wait()
	c.wg.Add(1)

	c.ctx, c.cancel = context.WithCancel(ctx)
	level.Info(c.logger).Log("msg", "starting consumer", "topics", fmt.Sprintf("%+v", topics))

	go func() {
		defer c.wg.Done()
		backoff := backoff.New(c.ctx, defaultBackOff)
		for {
			// Calling Consume in an infinite loop in case rebalancing is kicking in.
			// In which case all claims will be renewed.
			err := c.ConsumerGroup.Consume(c.ctx, topics, c)
			if err != nil && err != context.Canceled {
				level.Error(c.logger).Log("msg", "error from the consumer, retrying...", "err", err)
				// backoff before re-trying.
				backoff.Wait()
				if backoff.Ongoing() {
					continue
				}
				level.Error(c.logger).Log("msg", "maximum error from the consumer reached", "last_err", err)
				return
			}
			if c.ctx.Err() != nil || err == context.Canceled {
				level.Info(c.logger).Log("msg", "stopping consumer", "topics", fmt.Sprintf("%+v", topics))
				return
			}
			backoff.Reset()
		}
	}()
}

// ConsumeClaim creates a target for the given received claim and start reading message from it.
func (c *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	c.wg.Add(1)
	defer c.wg.Done()

	t, err := c.discoverer.NewTarget(session, claim)
	if err != nil {
		return err
	}
	if len(t.Labels()) == 0 {
		c.addDroppedTarget(t)
		t.run()
		return nil
	}
	c.addTarget(t)
	level.Info(c.logger).Log("msg", "consuming topic", "details", t.Details())
	t.run()

	return nil
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (c *consumer) Setup(session sarama.ConsumerGroupSession) error {
	c.resetTargets()
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (c *consumer) Cleanup(sarama.ConsumerGroupSession) error {
	c.resetTargets()
	return nil
}

// stop stops the consumer.
func (c *consumer) stop() {
	c.cancel()
	c.wg.Wait()
	c.resetTargets()
}

func (c *consumer) resetTargets() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.activeTargets = nil
	c.droppedTargets = nil
}

func (c *consumer) getActiveTargets() []Target {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.activeTargets
}

func (c *consumer) getDroppedTargets() []Target {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.droppedTargets
}

func (c *consumer) addTarget(t Target) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.activeTargets = append(c.activeTargets, t)
}

func (c *consumer) addDroppedTarget(t Target) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.droppedTargets = append(c.droppedTargets, t)
}
