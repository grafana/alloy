package opampmanager

import (
	"fmt"
	"slices"
	"strings"

	"github.com/open-telemetry/opamp-go/protobufs"
)

func OtelYAMLFromRemoteConfig(rc *protobufs.AgentRemoteConfig) ([]byte, error) {
	if rc == nil || rc.GetConfig() == nil {
		return nil, fmt.Errorf("opampmanager: remote config empty")
	}
	m := rc.GetConfig().GetConfigMap()
	if len(m) == 0 {
		return nil, fmt.Errorf("opampmanager: remote config config_map empty")
	}
	var lastErr error
	for _, k := range orderedConfigKeys(m) {
		b, err := fileBody(m[k])
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("opampmanager: remote config file missing body")
}

func orderedConfigKeys(m map[string]*protobufs.AgentConfigFile) []string {
	pref := []string{"config.yaml", "collector.yaml", "effective.yaml", "otel.yaml"}
	seen := make(map[string]struct{}, len(m))
	out := make([]string, 0, len(m))
	for _, p := range pref {
		if _, ok := m[p]; !ok {
			continue
		}
		out = append(out, p)
		seen[p] = struct{}{}
	}
	others := make([]string, 0, len(m))
	for k := range m {
		if _, ok := seen[k]; ok {
			continue
		}
		others = append(others, k)
	}
	slices.Sort(others)
	for i := len(others) - 1; i >= 0; i-- {
		k := others[i]
		if strings.HasSuffix(strings.ToLower(k), ".yaml") || strings.HasSuffix(strings.ToLower(k), ".yml") {
			out = append(out, k)
			seen[k] = struct{}{}
		}
	}
	for _, k := range others {
		if _, ok := seen[k]; ok {
			continue
		}
		out = append(out, k)
	}
	return out
}

func fileBody(f *protobufs.AgentConfigFile) ([]byte, error) {
	if f == nil || len(f.GetBody()) == 0 {
		return nil, fmt.Errorf("opampmanager: remote config file missing body")
	}
	return f.GetBody(), nil
}
