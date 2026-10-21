package common

import (
	"errors"
	"testing"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateKafkaTopics creates topics and waits until every partition has a leader.
// Producing to auto-created topics races leader election.
func CreateKafkaTopics(t *testing.T, brokers []string, cfg *sarama.Config, partitions int32, topics ...string) {
	t.Helper()

	admin, err := sarama.NewClusterAdmin(brokers, cfg)
	require.NoError(t, err)
	defer func() { require.NoError(t, admin.Close()) }()

	for _, topic := range topics {
		err := admin.CreateTopic(topic, &sarama.TopicDetail{NumPartitions: partitions, ReplicationFactor: 1}, false)
		if !errors.Is(err, sarama.ErrTopicAlreadyExists) {
			require.NoError(t, err)
		}
	}

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		metadata, err := admin.DescribeTopics(topics)
		require.NoError(c, err)

		for _, topic := range metadata {
			require.Equal(c, sarama.ErrNoError, topic.Err, "topic %s", topic.Name)
			require.Len(c, topic.Partitions, int(partitions), "topic %s", topic.Name)
			for _, p := range topic.Partitions {
				require.Equal(c, sarama.ErrNoError, p.Err, "topic %s partition %d", topic.Name, p.ID)
				require.GreaterOrEqual(c, p.Leader, int32(0), "topic %s partition %d", topic.Name, p.ID)
			}
		}
	}, DefaultTimeout, DefaultRetryInterval)
}
