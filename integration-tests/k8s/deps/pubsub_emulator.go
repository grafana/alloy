package deps

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

//go:embed manifests/pubsub-emulator.yaml
var pubSubEmulatorManifest string

const (
	// All must match manifests/pubsub-emulator.yaml.
	PubSubEmulatorProject      = "test-project"
	PubSubEmulatorTopic        = "gcplog-topic"
	PubSubEmulatorSubscription = "gcplog-subscription"

	pubSubEmulatorImage      = "messagebird/gcloud-pubsub-emulator:latest"
	pubSubEmulatorSelector   = "app=pubsub-emulator"
	pubSubEmulatorDeployment = "deployment/pubsub-emulator"
)

var _ harness.Dependency = (*PubSubEmulator)(nil)

// PubSubEmulator runs the Google Cloud Pub/Sub emulator with one topic and a
// pull subscription on it. In-cluster address: pubsub-emulator:8681, which
// clients reach by setting PUBSUB_EMULATOR_HOST.
type PubSubEmulator struct {
	opts      PubSubEmulatorOptions
	installed bool
}

type PubSubEmulatorOptions struct {
	Namespace string
}

func NewPubSubEmulator(opts PubSubEmulatorOptions) *PubSubEmulator {
	return &PubSubEmulator{opts: opts}
}

func (p *PubSubEmulator) Name() string { return "pubsub-emulator" }

func (p *PubSubEmulator) Install(_ *harness.TestContext) error {
	if p.opts.Namespace == "" {
		return errors.New("pubsub-emulator namespace is required")
	}
	if err := ensureKindImage(pubSubEmulatorImage); err != nil {
		return err
	}
	if err := util.Step("apply pubsub-emulator manifest", func() error {
		return harness.ApplyManifest(p.opts.Namespace, pubSubEmulatorManifest)
	}); err != nil {
		return err
	}
	p.installed = true

	return util.Step("wait for pubsub-emulator pod ready", func() error {
		return harness.WaitForReady(p.opts.Namespace, pubSubEmulatorSelector)
	})
}

func (p *PubSubEmulator) Cleanup() {
	if !p.installed {
		return
	}
	_ = harness.DeleteManifest(p.opts.Namespace, pubSubEmulatorManifest)
}

// Publish publishes each message to the topic through the emulator's REST
// API, from inside the emulator pod.
func (p *PubSubEmulator) Publish(t *testing.T, messages ...[]byte) {
	t.Helper()

	type message struct {
		Data string `json:"data"`
	}
	req := struct {
		Messages []message `json:"messages"`
	}{}
	for _, m := range messages {
		req.Messages = append(req.Messages, message{Data: base64.StdEncoding.EncodeToString(m)})
	}
	body, err := json.Marshal(req)
	require.NoError(t, err)

	publishURL := fmt.Sprintf("http://localhost:8681/v1/projects/%s/topics/%s:publish", PubSubEmulatorProject, PubSubEmulatorTopic)
	err = util.Step(fmt.Sprintf("publish %d messages to pubsub topic %s", len(messages), PubSubEmulatorTopic), func() error {
		return harness.RunCommandStdin(string(body), "kubectl",
			"--namespace", p.opts.Namespace, "exec", "-i", pubSubEmulatorDeployment, "--",
			"curl", "-sf", "-X", "POST", "-H", "Content-Type: application/json", "--data-binary", "@-", publishURL,
		)
	})
	require.NoError(t, err)
}
