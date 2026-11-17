package deps

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKafkaGenInstallRejectsMisconfiguration(t *testing.T) {
	ready := &Kafka{opts: KafkaOptions{Namespace: "ns", Topics: []string{"test_topic"}}, ready: true}
	notReady := &Kafka{opts: KafkaOptions{Namespace: "ns", Topics: []string{"test_topic"}}}

	tests := []struct {
		name    string
		opts    KafkaGenOptions
		wantErr string
	}{
		{"no kafka", KafkaGenOptions{Topic: "test_topic", Group: "g"}, "required"},
		{"kafka not installed yet", KafkaGenOptions{Kafka: notReady, Topic: "test_topic", Group: "g"}, "listed after"},
		{"topic kafka did not create", KafkaGenOptions{Kafka: ready, Topic: "other", Group: "g"}, "not one of"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorContains(t, NewKafkaGen(tt.opts).Install(nil), tt.wantErr)
		})
	}
}
