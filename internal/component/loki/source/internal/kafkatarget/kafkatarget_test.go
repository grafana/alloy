package kafkatarget

// This code is copied from Promtail (https://github.com/grafana/loki/commit/065bee7e72b00d800431f4b70f0d673d6e0e7a2b). The kafkatarget package is used to
// configure and run the targets that can read kafka entries and forward them
// to other loki components.

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki/client/fake"

	"github.com/IBM/sarama"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// Consumergroup handler
type testConsumerGroupHandler struct {
	handler sarama.ConsumerGroupHandler
	ctx     context.Context
	topics  []string

	returnErr error

	consuming atomic.Bool
	mut       sync.RWMutex
}

func (c *testConsumerGroupHandler) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	if c.returnErr != nil {
		return c.returnErr
	}

	c.mut.Lock()

	c.ctx = ctx
	c.topics = topics
	c.handler = handler

	c.mut.Unlock()

	c.consuming.Store(true)
	<-ctx.Done()
	c.consuming.Store(false)
	return nil
}

func (c *testConsumerGroupHandler) GetTopics() []string {
	c.mut.RLock()
	defer c.mut.RUnlock()

	return c.topics
}

func (c *testConsumerGroupHandler) Errors() <-chan error {
	return nil
}

func (c *testConsumerGroupHandler) Close() error {
	return nil
}

func (c *testConsumerGroupHandler) Pause(partitions map[string][]int32)  {}
func (c *testConsumerGroupHandler) Resume(partitions map[string][]int32) {}
func (c *testConsumerGroupHandler) PauseAll()                            {}
func (c *testConsumerGroupHandler) ResumeAll()                           {}

type testSession struct {
	markedMessage []*sarama.ConsumerMessage
}

func (s *testSession) Claims() map[string][]int32                                               { return nil }
func (s *testSession) MemberID() string                                                         { return "foo" }
func (s *testSession) GenerationID() int32                                                      { return 10 }
func (s *testSession) MarkOffset(topic string, partition int32, offset int64, metadata string)  {}
func (s *testSession) Commit()                                                                  {}
func (s *testSession) ResetOffset(topic string, partition int32, offset int64, metadata string) {}
func (s *testSession) MarkMessage(msg *sarama.ConsumerMessage, metadata string) {
	s.markedMessage = append(s.markedMessage, msg)
}
func (s *testSession) Context() context.Context { return context.Background() }

type testClaim struct {
	topic     string
	partition int32
	offset    int64
	messages  chan *sarama.ConsumerMessage
}

func newTestClaim(topic string, partition int32, offset int64) *testClaim {
	return &testClaim{
		topic:     topic,
		partition: partition,
		offset:    offset,
		messages:  make(chan *sarama.ConsumerMessage),
	}
}

func (t *testClaim) Topic() string                            { return t.topic }
func (t *testClaim) Partition() int32                         { return t.partition }
func (t *testClaim) InitialOffset() int64                     { return t.offset }
func (t *testClaim) HighWaterMarkOffset() int64               { return 0 }
func (t *testClaim) Messages() <-chan *sarama.ConsumerMessage { return t.messages }
func (t *testClaim) Send(m *sarama.ConsumerMessage) {
	t.messages <- m
}

func (t *testClaim) Stop() {
	close(t.messages)
}

func Test_TargetRun(t *testing.T) {
	tc := []struct {
		name            string
		inMessageKey    string
		inMessageOffset int64
		inLS            model.LabelSet
		inDiscoveredLS  model.LabelSet
		relabels        []*relabel.Config
		expectedLS      model.LabelSet
	}{
		{
			name:            "no relabel config",
			inMessageKey:    "foo",
			inMessageOffset: int64(42),
			inDiscoveredLS:  model.LabelSet{"__meta_kafka_foo": "bar"},
			inLS:            model.LabelSet{"buzz": "bazz"},
			relabels:        nil,
			expectedLS:      model.LabelSet{"buzz": "bazz"},
		},
		{
			name:            "message key with relabel config",
			inMessageKey:    "foo",
			inMessageOffset: int64(42),
			inDiscoveredLS:  model.LabelSet{"__meta_kafka_foo": "bar"},
			inLS:            model.LabelSet{"buzz": "bazz"},
			relabels: []*relabel.Config{
				{
					SourceLabels:         model.LabelNames{"__meta_kafka_message_key"},
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "message_key",
					Replacement:          "$1",
					Action:               "replace",
					NameValidationScheme: model.LegacyValidation,
				},
			},
			expectedLS: model.LabelSet{"buzz": "bazz", "message_key": "foo"},
		},
		{
			name:            "no message key with relabel config",
			inMessageKey:    "",
			inMessageOffset: 42,
			inDiscoveredLS:  model.LabelSet{"__meta_kafka_foo": "bar"},
			inLS:            model.LabelSet{"buzz": "bazz"},
			relabels: []*relabel.Config{
				{
					SourceLabels:         model.LabelNames{"__meta_kafka_message_key"},
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "message_key",
					Replacement:          "$1",
					Action:               "replace",
					NameValidationScheme: model.LegacyValidation,
				},
			},
			expectedLS: model.LabelSet{"buzz": "bazz", "message_key": "none"},
		},
		{
			name:            "message offset with relabel config",
			inMessageKey:    "foo",
			inMessageOffset: 42,
			inDiscoveredLS:  model.LabelSet{"__meta_kafka_foo": "bar"},
			inLS:            model.LabelSet{"buzz": "bazz"},
			relabels: []*relabel.Config{
				{
					SourceLabels:         model.LabelNames{"__meta_kafka_message_offset"},
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "message_offset",
					Replacement:          "$1",
					Action:               "replace",
					NameValidationScheme: model.LegacyValidation,
				},
			},
			expectedLS: model.LabelSet{"buzz": "bazz", "message_offset": "42"},
		},
		{
			name:            "no message offset with relabel config",
			inMessageKey:    "",
			inMessageOffset: int64(0),
			inDiscoveredLS:  model.LabelSet{"__meta_kafka_foo": "bar"},
			inLS:            model.LabelSet{"buzz": "bazz"},
			relabels: []*relabel.Config{
				{
					SourceLabels:         model.LabelNames{"__meta_kafka_message_offset"},
					Regex:                relabel.MustNewRegexp("(.*)"),
					TargetLabel:          "message_offset",
					Replacement:          "$1",
					Action:               "replace",
					NameValidationScheme: model.LegacyValidation,
				},
			},
			expectedLS: model.LabelSet{"buzz": "bazz", "message_offset": "0"},
		},
	}
	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			session, claim := &testSession{}, newTestClaim("footopic", 10, 12)
			var closed bool
			fc := fake.NewClient(
				func() {
					closed = true
				},
			)

			tg := NewKafkaTarget(nil, session, claim, tt.inDiscoveredLS, tt.inLS, tt.relabels, fc, true, &KafkaTargetMessageParser{})

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				tg.run()
			}()

			for i := 0; i < 10; i++ {
				claim.Send(&sarama.ConsumerMessage{
					Timestamp: time.Unix(0, int64(i)),
					Value:     []byte(fmt.Sprintf("%d", i)),
					Key:       []byte(tt.inMessageKey),
					Offset:    tt.inMessageOffset,
				})
			}
			claim.Stop()
			wg.Wait()
			re := fc.Received()

			require.Len(t, session.markedMessage, 10)
			require.Len(t, re, 10)
			require.True(t, closed)
			for _, e := range re {
				require.Equal(t, tt.expectedLS.String(), e.Labels.String())
			}
		})
	}
}
