package deps

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	// All must match manifests/nginx-proxy.yaml.
	nginxSelector = "app=nginx-proxy"
	nginxName     = "nginx-proxy"
	nginxImage    = "nginx:1.27-alpine"
)

//go:embed manifests/nginx-proxy.yaml
var nginxProxyManifest string

// Compile-time check that *NginxProxy satisfies the harness.Dependency interface.
var _ harness.Dependency = (*NginxProxy)(nil)

// NginxProxy runs an nginx reverse proxy in front of Mimir's push path. It logs
// the User-Agent of every /api/v1/push request as JSON on stdout and forwards
// the request to http://mimir:9009. In-cluster URL: http://nginx-proxy:8080.
// Install it after Mimir so the upstream Service exists when nginx starts.
type NginxProxy struct {
	opts      NginxProxyOptions
	namespace string
	installed bool
}

type NginxProxyOptions struct {
	Namespace string
}

func NewNginxProxy(opts NginxProxyOptions) *NginxProxy {
	return &NginxProxy{opts: opts, namespace: opts.Namespace}
}

func (n *NginxProxy) Name() string { return nginxName }

func (n *NginxProxy) Install(ctx *harness.TestContext) error {
	if n.namespace == "" {
		return fmt.Errorf("nginx-proxy namespace is required")
	}

	if err := ensureKindImage(nginxImage); err != nil {
		return fmt.Errorf("failed to load nginx image: %w", err)
	}

	if err := util.Step("apply nginx-proxy manifest", func() error {
		return harness.ApplyManifest(n.namespace, nginxProxyManifest)
	}); err != nil {
		return err
	}
	n.installed = true

	if err := util.Step("wait for nginx-proxy pod ready", func() error {
		return harness.WaitForReady(n.namespace, nginxSelector)
	}); err != nil {
		return err
	}

	ctx.AddDiagnosticHook("nginx-proxy logs", func(c context.Context) error {
		return harness.RunDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", n.namespace, "logs", "-l", nginxSelector, "--all-containers=true", "--tail", "200"},
		})
	})
	return nil
}

func (n *NginxProxy) Cleanup() {
	if !n.installed || n.namespace == "" {
		return
	}
	_ = harness.DeleteManifest(n.namespace, nginxProxyManifest)
}

// pushLogLine matches the escape=json access log format in
// manifests/nginx-proxy.yaml.
type pushLogLine struct {
	UserAgent string `json:"user_agent"`
}

// PushUserAgents reads the proxy's stdout and returns the User-Agent of every
// logged /api/v1/push request, in order. Non-JSON lines (e.g. nginx error_log
// output) are skipped.
func (n *NginxProxy) PushUserAgents() ([]string, error) {
	out, err := harness.RunCommandOutput("kubectl", "--namespace", n.namespace, "logs", "deploy/"+nginxName, "--tail=-1")
	if err != nil {
		return nil, err
	}

	var userAgents []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry pushLogLine
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.UserAgent != "" {
			userAgents = append(userAgents, entry.UserAgent)
		}
	}
	return userAgents, nil
}
