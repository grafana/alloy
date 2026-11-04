package deps

import (
	_ "embed"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/kafka-gen.yaml
var kafkaGenManifest string

const (
	kafkaGenSelector         = "app=kafka-gen"
	kafkaGenTopicPlaceholder = "__KAFKA_GEN_TOPIC__"
	kafkaGenGroupPlaceholder = "__KAFKA_GEN_GROUP__"
)

var _ harness.Dependency = (*KafkaGen)(nil)

// KafkaGen produces a record a second to a topic of a Kafka dependency and
// consumes them in a consumer group, so the group has offsets and lag. List it
// after that Kafka dependency.
type KafkaGen struct {
	opts      KafkaGenOptions
	manifest  string
	installed bool
}

type KafkaGenOptions struct {
	// Kafka is the broker to run against. KafkaGen runs in its namespace.
	Kafka *Kafka
	// Topic must be one of Kafka's KafkaOptions.Topics.
	Topic string
	Group string
}

func NewKafkaGen(opts KafkaGenOptions) *KafkaGen {
	return &KafkaGen{opts: opts}
}

func (k *KafkaGen) Name() string { return "kafka-gen" }

func (k *KafkaGen) Install(_ *harness.TestContext) error {
	kafka := k.opts.Kafka
	if kafka == nil || k.opts.Topic == "" || k.opts.Group == "" {
		return errors.New("kafka-gen kafka, topic and group are required")
	}
	// A ready Kafka has also loaded the image kafka-gen runs.
	if !kafka.ready {
		return errors.New("kafka-gen must be listed after its kafka dependency")
	}
	// The broker auto-creates unknown topics with one partition and no leader
	// wait, so require a topic Kafka created.
	if !slices.Contains(kafka.opts.Topics, k.opts.Topic) {
		return fmt.Errorf("kafka-gen topic %q is not one of the kafka dependency's topics %v", k.opts.Topic, kafka.opts.Topics)
	}
	k.manifest = strings.NewReplacer(
		kafkaGenTopicPlaceholder, k.opts.Topic,
		kafkaGenGroupPlaceholder, k.opts.Group,
	).Replace(kafkaGenManifest)
	if err := util.Step("apply kafka-gen manifest", func() error {
		return harness.ApplyManifest(kafka.opts.Namespace, k.manifest)
	}); err != nil {
		return err
	}
	k.installed = true
	return util.Step("wait for kafka-gen pod ready", func() error {
		return harness.WaitForReady(kafka.opts.Namespace, kafkaGenSelector)
	})
}

func (k *KafkaGen) Cleanup() {
	if !k.installed {
		return
	}
	_ = harness.DeleteManifest(k.opts.Kafka.opts.Namespace, k.manifest)
}
