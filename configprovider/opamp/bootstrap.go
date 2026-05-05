// Copyright Grafana Labs and OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opamp

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/google/uuid"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/config/configtls"
	"google.golang.org/protobuf/proto"
	yamlv3 "gopkg.in/yaml.v3"
)

//go:embed extension_template.yaml
var extensionTemplate string

const lastRecvRemoteConfigFile = "last_recv_remote_config.dat"

// BootstrapOpamp is the top-level `opamp` block from the user's bootstrap YAML.
type BootstrapOpamp struct {
	Server  RemoteServer          `yaml:"server" mapstructure:"server"`
	Storage StorageSettings       `yaml:"storage" mapstructure:"storage"`
	Agent   AgentOverrideSettings `yaml:"agent" mapstructure:"agent"`
}

// RemoteServer configures the outbound OpAMP connection to the management server.
type RemoteServer struct {
	Endpoint   string                 `yaml:"endpoint" mapstructure:"endpoint"`
	Headers    map[string]any         `yaml:"headers,omitempty" mapstructure:"headers"`
	ClientAuth *ClientAuthSettings    `yaml:"client_auth,omitempty" mapstructure:"client_auth,omitempty"`
	TLS        configtls.ClientConfig `yaml:"tls,omitempty" mapstructure:"tls"`
}

type ClientAuthSettings struct {
	Username string `yaml:"username" mapstructure:"username"`
	Password string `yaml:"password" mapstructure:"password"`
}

// StorageSettings for instance id and last-received remote config.
type StorageSettings struct {
	Directory string `yaml:"directory" mapstructure:"directory"`
}

// AgentOverrideSettings mirrors the supervisor's agent.opamp_server_port.
type AgentOverrideSettings struct {
	OpAMPServerPort int `yaml:"opamp_server_port" mapstructure:"opamp_server_port"`
}

func readAndParseBootstrap(path string) (mgmt *BootstrapOpamp, collectorRoot map[string]any, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("opamp bootstrap: abs path: %w", err)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return nil, nil, fmt.Errorf("opamp bootstrap: read %s: %w", abs, err)
	}
	var root map[string]any
	if err := yamlv3.Unmarshal(b, &root); err != nil {
		return nil, nil, fmt.Errorf("opamp bootstrap: yaml: %w", err)
	}
	rawOpamp, hasOpamp := root["opamp"]
	if !hasOpamp {
		return nil, nil, fmt.Errorf("opamp bootstrap: missing required top-level `opamp` block")
	}
	opampBytes, err := yamlv3.Marshal(rawOpamp)
	if err != nil {
		return nil, nil, err
	}
	var bo BootstrapOpamp
	if err := yamlv3.Unmarshal(opampBytes, &bo); err != nil {
		return nil, nil, fmt.Errorf("opamp bootstrap: opamp block: %w", err)
	}
	if err := validateBootstrapMgmt(&bo); err != nil {
		return nil, nil, err
	}
	delete(root, "opamp")
	return &bo, root, nil
}

func validateBootstrapMgmt(bo *BootstrapOpamp) error {
	if bo == nil {
		return fmt.Errorf("opamp bootstrap: nil management config")
	}
	if strings.TrimSpace(bo.Server.Endpoint) == "" {
		return fmt.Errorf("opamp.server.endpoint is required")
	}
	if strings.TrimSpace(bo.Storage.Directory) == "" {
		return fmt.Errorf("opamp.storage.directory is required")
	}
	if bo.Agent.OpAMPServerPort < 0 || bo.Agent.OpAMPServerPort > 65535 {
		return fmt.Errorf("opamp.agent.opamp_server_port must be between 0 and 65535")
	}
	if ca := bo.Server.ClientAuth; ca != nil {
		if strings.TrimSpace(ca.Username) == "" || strings.TrimSpace(ca.Password) == "" {
			return fmt.Errorf("opamp.server.client_auth requires non-empty username and password")
		}
		if strings.Contains(ca.Username, ":") {
			return fmt.Errorf("opamp.server.client_auth.username must not contain ':'")
		}
	}
	return bo.Server.TLS.Validate()
}

func needsDefaultShell(root map[string]any) bool {
	if root == nil {
		return true
	}
	svc, ok := root["service"].(map[string]any)
	if !ok {
		return true
	}
	pipes, ok := svc["pipelines"].(map[string]any)
	if !ok || len(pipes) == 0 {
		return true
	}
	return false
}

var defaultShellYAML = []byte(`receivers:
  otlp:
    protocols:
      grpc:

exporters:
  nop:

service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [nop]
`)

type extensionTemplateData struct {
	InstanceUid string
	LocalPort   int
}

func renderExtensionYAML(instanceID uuid.UUID, localPort int) ([]byte, error) {
	tpl, err := template.New("opamp-extension").Parse(extensionTemplate)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, extensionTemplateData{
		InstanceUid: instanceID.String(),
		LocalPort:   localPort,
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Compose merges user YAML (without opamp key), optional remote AgentRemoteConfig, and the injected extension.
func Compose(
	userCollector map[string]any,
	remote *protobufs.AgentRemoteConfig,
	instanceID uuid.UUID,
	localPort int,
) ([]byte, error) {
	var merged map[string]any

	if needsDefaultShell(userCollector) {
		if err := yamlv3.Unmarshal(defaultShellYAML, &merged); err != nil {
			return nil, fmt.Errorf("parse default shell: %w", err)
		}
	} else {
		merged = make(map[string]any)
	}

	if len(userCollector) > 0 {
		uc, ok := deepCloneValue(userCollector).(map[string]any)
		if !ok {
			return nil, fmt.Errorf("user collector root must be a map")
		}
		mergeRoot(merged, uc)
	}

	extBytes, err := renderExtensionYAML(instanceID, localPort)
	if err != nil {
		return nil, err
	}
	var ext map[string]any
	if err := yamlv3.Unmarshal(extBytes, &ext); err != nil {
		return nil, fmt.Errorf("parse extension template: %w", err)
	}
	mergeRoot(merged, ext)

	if remote != nil && len(remote.GetConfig().GetConfigMap()) > 0 {
		for _, name := range sortedRemoteConfigNames(remote.GetConfig()) {
			item := remote.GetConfig().GetConfigMap()[name]
			if item == nil || len(item.Body) == 0 {
				continue
			}
			var layer map[string]any
			if err := yamlv3.Unmarshal(item.Body, &layer); err != nil {
				return nil, fmt.Errorf("merge remote config %q: %w", name, err)
			}
			mergeRoot(merged, layer)
		}
	}

	return yamlv3.Marshal(merged)
}

func sortedRemoteConfigNames(cfg *protobufs.AgentConfigMap) []string {
	if cfg == nil || cfg.ConfigMap == nil {
		return nil
	}
	var names []string
	for name := range cfg.ConfigMap {
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	names = append(names, "")
	return names
}

// SaveLastReceivedRemoteConfig persists protobuf AgentRemoteConfig like the supervisor.
func SaveLastReceivedRemoteConfig(storageDir string, config *protobufs.AgentRemoteConfig) error {
	cfg, err := proto.Marshal(config)
	if err != nil {
		return err
	}
	path := filepath.Join(storageDir, lastRecvRemoteConfigFile)
	return os.WriteFile(path, cfg, 0o600)
}

// LoadLastReceivedRemoteConfig loads persisted remote config if present.
func LoadLastReceivedRemoteConfig(storageDir string) (*protobufs.AgentRemoteConfig, error) {
	path := filepath.Join(storageDir, lastRecvRemoteConfigFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c protobufs.AgentRemoteConfig
	if err := proto.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func parseURI(uri string) (path string, err error) {
	const prefix = "opamp:"
	if !strings.HasPrefix(uri, prefix) {
		return "", fmt.Errorf("uri must have scheme opamp:")
	}
	path = strings.TrimSpace(strings.TrimPrefix(uri, prefix))
	if path == "" {
		return "", fmt.Errorf("opamp: URI missing path")
	}
	return filepath.Clean(path), nil
}
