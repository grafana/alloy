//go:build alloyintegrationtests

package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/grafana/alloy/integration-tests/docker/common"
)

var topics = []string{"test1", "test2", "test3"}

const (
	messagesPerTopic = 50
	broker           = "localhost:29093"
	caPath           = "certs/server.crt"
)

func TestLokiAzureEventHubs(t *testing.T) {
	producer, err := newProducer(t.Context())
	require.NoError(t, err)
	defer func() { require.NoError(t, producer.Close()) }()

	var g errgroup.Group
	for _, tp := range topics {
		g.Go(func() error { return sendMessages(producer, tp, messagesPerTopic) })
	}
	require.NoError(t, g.Wait())

	total := messagesPerTopic * len(topics)
	assertions := make([]common.ExpectedLogResult, 0, len(topics))

	for _, tp := range topics {
		assertions = append(assertions, common.ExpectedLogResult{
			Labels: map[string]string{
				"service_name": tp,
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

func newProducer(ctx context.Context) (sarama.SyncProducer, error) {
	caBytes, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert at %q: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("failed to parse CA cert at %q", caPath)
	}

	cfg := sarama.NewConfig()
	cfg.Version = sarama.V1_0_0_0
	cfg.Producer.Return.Successes = true
	cfg.Net.SASL.Enable = true
	cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	cfg.Net.SASL.User = "$ConnectionString"
	cfg.Net.SASL.Password = "fake-connection-string"
	cfg.Net.TLS.Enable = true
	cfg.Net.TLS.Config = &tls.Config{RootCAs: pool}

	bk := backoff.New(ctx, backoff.Config{
		MinBackoff: 50 * time.Millisecond,
		MaxBackoff: 5 * time.Second,
		MaxRetries: 10,
	})

	var (
		lastErr  error
		producer sarama.SyncProducer
	)

	for bk.Ongoing() {
		producer, lastErr = sarama.NewSyncProducer([]string{broker}, cfg)
		if lastErr == nil {
			return producer, nil
		}

		bk.Wait()
	}

	return nil, fmt.Errorf("failed to connect to kafka: %w", lastErr)
}

func sendMessages(producer sarama.SyncProducer, topic string, n int) error {
	for i := range n {
		payload := fmt.Sprintf(
			`{"records":[{"time":%q,"category":%q,"resourceId":"/subscriptions/test/resourceGroups/rg/providers/Microsoft.Example/x/y","operationName":"Write","msg":"log line %d"}]}`,
			time.Now().UTC().Format(time.RFC3339),
			topic,
			i,
		)
		_, _, err := producer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.StringEncoder(payload),
		})
		if err != nil {
			return fmt.Errorf("failed to send message to topic %q: %w", topic, err)
		}
	}

	return nil
}
