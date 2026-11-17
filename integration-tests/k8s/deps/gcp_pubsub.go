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

//go:embed manifests/gcp-pubsub.yaml
var gcpPubSubManifest string

const (
	// All must match manifests/gcp-pubsub.yaml.
	GCPPubSubProject      = "test-project"
	GCPPubSubTopic        = "gcplog-topic"
	GCPPubSubSubscription = "gcplog-subscription"

	gcpPubSubImage      = "thekevjames/gcloud-pubsub-emulator:592.0.0"
	gcpPubSubSelector   = "app=gcp-pubsub"
	gcpPubSubDeployment = "deployment/gcp-pubsub"
)

var _ harness.Dependency = (*GCPPubSub)(nil)

// GCPPubSub runs the GCP Pub/Sub emulator with one topic and a
// pull subscription on it. In-cluster address: gcp-pubsub:8681, which
// clients reach by setting PUBSUB_EMULATOR_HOST.
type GCPPubSub struct {
	opts      GCPPubSubOptions
	installed bool
}

type GCPPubSubOptions struct {
	Namespace string
}

func NewGCPPubSub(opts GCPPubSubOptions) *GCPPubSub {
	return &GCPPubSub{opts: opts}
}

func (p *GCPPubSub) Name() string { return "gcp-pubsub" }

func (p *GCPPubSub) Install(_ *harness.TestContext) error {
	if p.opts.Namespace == "" {
		return errors.New("gcp-pubsub namespace is required")
	}
	if err := ensureKindImage(gcpPubSubImage); err != nil {
		return err
	}
	if err := util.Step("apply gcp-pubsub manifest", func() error {
		return harness.ApplyManifest(p.opts.Namespace, gcpPubSubManifest)
	}); err != nil {
		return err
	}
	p.installed = true

	return util.Step("wait for gcp-pubsub pod ready", func() error {
		return harness.WaitForReady(p.opts.Namespace, gcpPubSubSelector)
	})
}

func (p *GCPPubSub) Cleanup() {
	if !p.installed {
		return
	}
	_ = harness.DeleteManifest(p.opts.Namespace, gcpPubSubManifest)
}

// Publish publishes each message to the topic through the emulator's REST
// API, from inside the emulator pod.
func (p *GCPPubSub) Publish(t *testing.T, messages ...[]byte) {
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

	publishURL := fmt.Sprintf("http://localhost:8681/v1/projects/%s/topics/%s:publish", GCPPubSubProject, GCPPubSubTopic)
	err = util.Step(fmt.Sprintf("publish %d messages to GCP Pub/Sub topic %s", len(messages), GCPPubSubTopic), func() error {
		return harness.RunCommandStdin(string(body), "kubectl",
			"--namespace", p.opts.Namespace, "exec", "-i", gcpPubSubDeployment, "--",
			"curl", "-sf", "-X", "POST", "-H", "Content-Type: application/json", "--data-binary", "@-", publishURL,
		)
	})
	require.NoError(t, err)
}
