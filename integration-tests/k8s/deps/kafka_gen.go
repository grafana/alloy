package deps

import (
	_ "embed"
	"fmt"
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

// KafkaGen produces a record a second to a topic of the Kafka dependency and
// consumes them in a consumer group, so the group has offsets and lag.
type KafkaGen struct {
	opts      KafkaGenOptions
	manifest  string
	installed bool
}

type KafkaGenOptions struct {
	Namespace string
	// Topic must already exist. Create it with KafkaOptions.Topics.
	Topic string
	Group string
}

func NewKafkaGen(opts KafkaGenOptions) *KafkaGen {
	return &KafkaGen{opts: opts}
}

func (k *KafkaGen) Name() string { return "kafka-gen" }

func (k *KafkaGen) Install(_ *harness.TestContext) error {
	if k.opts.Namespace == "" || k.opts.Topic == "" || k.opts.Group == "" {
		return fmt.Errorf("kafka-gen namespace, topic and group are required")
	}
	// The Kafka dependency loads the same image, so this is normally a no-op.
	if err := ensureKindImage(kafkaImage); err != nil {
		return fmt.Errorf("failed to load kafka image: %w", err)
	}
	k.manifest = strings.NewReplacer(
		kafkaGenTopicPlaceholder, k.opts.Topic,
		kafkaGenGroupPlaceholder, k.opts.Group,
	).Replace(kafkaGenManifest)
	if err := util.Step("apply kafka-gen manifest", func() error {
		return harness.ApplyManifest(k.opts.Namespace, k.manifest)
	}); err != nil {
		return err
	}
	k.installed = true
	return util.Step("wait for kafka-gen pod ready", func() error {
		return harness.WaitForReady(k.opts.Namespace, kafkaGenSelector)
	})
}

func (k *KafkaGen) Cleanup() {
	if !k.installed {
		return
	}
	_ = harness.DeleteManifest(k.opts.Namespace, k.manifest)
}
