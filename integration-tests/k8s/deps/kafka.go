package deps

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

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

	kafkaLeaderTimeout      = time.Minute
	kafkaLeaderPollInterval = time.Second
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
	// Topics are created once the broker is ready, so consumers can join their
	// group before any records are produced.
	Topics []string
	// Partitions is the partition count of each topic. Defaults to 1.
	Partitions int
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

	partitions := max(k.opts.Partitions, 1)
	for _, topic := range k.opts.Topics {
		if err := util.Step("create kafka topic "+topic, func() error {
			return k.exec("",
				kafkaBinDir+"/kafka-topics.sh",
				"--bootstrap-server", kafkaBootstrap,
				"--create", "--if-not-exists",
				"--topic", topic,
				"--partitions", strconv.Itoa(partitions),
				"--replication-factor", "1",
			)
		}); err != nil {
			return err
		}
		// Producing before every partition has a leader fails with "not the
		// leader for some partition".
		if err := util.Step("wait for kafka topic "+topic+" leaders", func() error {
			return k.waitForLeaders(topic, partitions)
		}); err != nil {
			return err
		}
	}
	return nil
}

// waitForLeaders polls the topic description until it lists partitions
// partitions, each with a leader.
func (k *Kafka) waitForLeaders(topic string, partitions int) error {
	deadline := time.Now().Add(kafkaLeaderTimeout)
	var lastErr error
	for time.Now().Before(deadline) {
		out, err := harness.RunCommandOutput("kubectl",
			"--namespace", k.opts.Namespace, "exec", kafkaDeployment, "--",
			kafkaBinDir+"/kafka-topics.sh",
			"--bootstrap-server", kafkaBootstrap,
			"--describe", "--topic", topic,
		)
		if err == nil {
			err = checkLeaders(out, partitions)
		}
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(kafkaLeaderPollInterval)
	}
	return fmt.Errorf("timed out after %s waiting for leaders of kafka topic %s: %w", kafkaLeaderTimeout, topic, lastErr)
}

// checkLeaders parses `kafka-topics.sh --describe` output, which has one line
// per partition such as "Topic: t\tPartition: 0\tLeader: 1\tReplicas: 1 ...".
func checkLeaders(describe string, partitions int) error {
	var withLeader int
	for _, line := range strings.Split(describe, "\n") {
		fields := strings.Fields(line)
		if !slices.Contains(fields, "Partition:") {
			continue
		}
		i := slices.Index(fields, "Leader:")
		if i < 0 || i+1 >= len(fields) {
			return fmt.Errorf("no leader field in %q", line)
		}
		if leader, err := strconv.Atoi(fields[i+1]); err != nil || leader < 0 {
			return fmt.Errorf("partition has no leader: %q", line)
		}
		withLeader++
	}
	if withLeader != partitions {
		return fmt.Errorf("found %d of %d partitions with a leader", withLeader, partitions)
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
	k.produce(t, topic, messages, nil)
}

// KafkaRecord is a keyed Kafka record. Records with different keys are spread
// across a topic's partitions.
type KafkaRecord struct {
	Key   string
	Value string
}

// ProduceKeyed writes each record to topic, partitioned by its key. Keys must
// not contain a tab, and neither keys nor values may contain newlines.
func (k *Kafka) ProduceKeyed(t *testing.T, topic string, records ...KafkaRecord) {
	t.Helper()

	lines := make([]string, 0, len(records))
	for _, r := range records {
		require.NotContains(t, r.Key, "\t", "kafka keys must not contain a tab")
		lines = append(lines, r.Key+"\t"+r.Value)
	}
	k.produce(t, topic, lines, []string{"--property", "parse.key=true"})
}

func (k *Kafka) produce(t *testing.T, topic string, lines, extraArgs []string) {
	t.Helper()

	for _, l := range lines {
		require.NotContains(t, l, "\n", "kafka messages must be single-line")
	}

	err := util.Step(fmt.Sprintf("produce %d records to kafka topic %s", len(lines), topic), func() error {
		args := []string{
			kafkaBinDir + "/kafka-console-producer.sh",
			"--bootstrap-server", kafkaBootstrap,
			"--topic", topic,
		}
		return k.exec(strings.Join(lines, "\n")+"\n", append(args, extraArgs...)...)
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
