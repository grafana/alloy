package alloyengine

import (
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gopkg.in/yaml.v2"
)

const persistentStateFilename = "persistent_state.yaml"

type persistentStateYAML struct {
	InstanceID string `yaml:"instance_id"`
}

// ReadPersistentStateInstanceID reads instance_id from persistent_state.yaml under dir
// (e.g. opampsupervisor storage). Returns empty string if missing or invalid.
func ReadPersistentStateInstanceID(dir string) string {
	if dir == "" {
		return ""
	}
	path := filepath.Join(dir, persistentStateFilename)
	if !fileExists(path) {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var ps persistentStateYAML
	if err := yaml.Unmarshal(data, &ps); err != nil {
		return ""
	}
	if ps.InstanceID == "" {
		return ""
	}
	parsed, err := uuid.Parse(ps.InstanceID)
	if err != nil {
		return ""
	}
	return parsed.String()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
