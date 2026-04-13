package opampprovider // import "github.com/grafana/alloy/configprovider/opampprovider"

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// readBootstrapYAML reads and unmarshals the bootstrap config file at basePath.
// E.g. If you run `alloy otel --config=opamp:bootstrap.yaml`, this method parses bootstrap.yaml
func readBootstrapYAML(basePath string) ([]byte, map[string]any, error) {
	baseBytes, err := os.ReadFile(basePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read base config %s: %w", basePath, err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(baseBytes, &root); err != nil {
		return nil, nil, fmt.Errorf("parse base yaml %s: %w", basePath, err)
	}
	return baseBytes, root, nil
}

// opampExtensionPaths returns remote_configuration_directory from the bootstrap root map.
func opampExtensionPaths(root map[string]any, basePath string) (remoteDir string, err error) {
	remoteDir, err = remoteConfigurationDirectory(root)
	if err != nil {
		return "", fmt.Errorf("base %s: %w", basePath, err)
	}
	if !filepath.IsAbs(remoteDir) {
		return "", fmt.Errorf("base %s: extensions.opamp.remote_configuration_directory must be an absolute path, got %q", basePath, remoteDir)
	}
	return remoteDir, nil
}
