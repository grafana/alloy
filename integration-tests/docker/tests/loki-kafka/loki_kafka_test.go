//go:build alloyintegrationtests

package main

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/docker/common"
	"github.com/grafana/dskit/backoff"
)

var topics = []string{"test1", "test2", "test3"}

const (
	messagesPerTopic = 50
	broker           = "localhost:29092"
)

func TestLokiKafka(t *testing.T) {
	producer, err := newProducer()
	require.NoError(t, err)
	defer func() { require.NoError(t, producer.Close()) }()

	var (
		wg   sync.WaitGroup
		errs = make([]error, len(topics))
	)

	for i, t := range topics {
		wg.Go(func() { errs[i] = sendMessages(producer, t, messagesPerTopic) })
	}

	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}

	var (
		total      = messagesPerTopic * len(topics)
		assertions = make([]common.ExpectedLogResult, 0, len(topics))
	)

	for _, t := range topics {
		assertions = append(assertions, common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": t,
			},
			EntryCount: messagesPerTopic,
		})
	}

	common.AssertLogsPresent(
		t,
		total,
		assertions...,
	)
}

func newProducer() (sarama.SyncProducer, error) {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	backoff := backoff.New(context.Background(), backoff.Config{
		MinBackoff: 50 * time.Millisecond,
		MaxBackoff: 5 * time.Second,
		MaxRetries: 10,
	})

	var (
		lastErr  error
		producer sarama.SyncProducer
	)

	for backoff.Ongoing() {
		producer, lastErr = sarama.NewSyncProducer([]string{broker}, config)
		if lastErr == nil {
			return producer, nil
		}

		backoff.Wait()
	}

	return nil, fmt.Errorf("failed to connect to kafka: %w", lastErr)
}

func sendMessages(producer sarama.SyncProducer, topic string, n int) error {
	for i := range n {
		_, _, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder("log line " + strconv.Itoa(i)),
		})
		if err != nil {
			return fmt.Errorf("failed to send message to topic %q: %w", topic, err)
		}
	}

	return nil
}
