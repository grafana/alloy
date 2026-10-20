package deps

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	// All must match manifests/kafka.yaml.
	kafkaSelector   = "app=kafka"
	kafkaDeployment = "deployment/kafka"
	kafkaBinDir     = "/opt/kafka/bin"

	// kafkaBootstrap is used by the Kafka CLI tools inside the broker pod.
	kafkaBootstrap = "localhost:9092"

	kafkaImage = "apache/kafka:4.1.0"
)

//go:embed manifests/kafka.yaml
var kafkaManifest string

var _ harness.Dependency = (*Kafka)(nil)

// Kafka runs a single-node Kafka broker in KRaft mode.
// In-cluster bootstrap address: kafka:9092.
type Kafka struct {
	opts      KafkaOptions
	installed bool
}

type KafkaOptions struct {
	Namespace string
	// Topics are created with a single partition once the broker is ready, so
	// consumers can join their group before any records are produced.
	Topics []string
}

func NewKafka(opts KafkaOptions) *Kafka {
	return &Kafka{opts: opts}
}

func (k *Kafka) Name() string { return "kafka" }

func (k *Kafka) Install(_ *harness.TestContext) (err error) {
	if k.opts.Namespace == "" {
		return errors.New("kafka namespace is required")
	}

	if err := ensureKindImage(kafkaImage); err != nil {
		return fmt.Errorf("failed to load kafka image: %w", err)
	}

	// The harness only registers a dependency after a successful Install, so
	// remove anything applied (even partially) if a later step fails.
	k.installed = true
	defer func() {
		if err != nil {
			k.Cleanup()
		}
	}()

	if err := util.Step("apply kafka manifest", func() error {
		return harness.ApplyManifest(k.opts.Namespace, kafkaManifest)
	}); err != nil {
		return err
	}

	if err := util.Step("wait for kafka pod ready", func() error {
		return harness.WaitForReady(k.opts.Namespace, kafkaSelector)
	}); err != nil {
		return err
	}

	for _, topic := range k.opts.Topics {
		if err := util.Step("create kafka topic "+topic, func() error {
			return k.exec("",
				kafkaBinDir+"/kafka-topics.sh",
				"--bootstrap-server", kafkaBootstrap,
				"--create", "--if-not-exists",
				"--topic", topic,
				"--partitions", "1",
				"--replication-factor", "1",
			)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kafka) Cleanup() {
	if !k.installed {
		return
	}
	_ = harness.DeleteManifest(k.opts.Namespace, kafkaManifest)
}

// Produce writes each message as one record to topic. Messages must not
// contain newlines, since the console producer reads one record per line.
func (k *Kafka) Produce(t *testing.T, topic string, messages ...string) {
	t.Helper()

	for _, m := range messages {
		require.NotContains(t, m, "\n", "kafka messages must be single-line")
	}

	err := util.Step(fmt.Sprintf("produce %d records to kafka topic %s", len(messages), topic), func() error {
		return k.exec(strings.Join(messages, "\n")+"\n",
			kafkaBinDir+"/kafka-console-producer.sh",
			"--bootstrap-server", kafkaBootstrap,
			"--topic", topic,
		)
	})
	require.NoError(t, err)
}

// exec runs a command in the broker pod, piping stdin to it when non-empty.
func (k *Kafka) exec(stdin string, command ...string) error {
	args := []string{"--namespace", k.opts.Namespace, "exec"}
	if stdin != "" {
		args = append(args, "-i")
	}
	args = append(args, kafkaDeployment, "--")
	args = append(args, command...)
	if stdin == "" {
		return harness.RunCommand("kubectl", args...)
	}
	return harness.RunCommandStdin(stdin, "kubectl", args...)
}
