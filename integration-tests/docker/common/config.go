package common

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TestConfig is used by tests to describe special setup requirements.
type TestConfig struct {
	Container ContainerConfig `yaml:"container"`
}

// ContainerConfig configures the container used for the test.
type ContainerConfig struct {
	// Image is the container image (org/name:tag). When set, the runner starts that image instead
	// of the default Alloy container.
	Image string `yaml:"image"`
	// SelfManaged when true signals that the test manages its own container lifecycle.
	// The runner will not start any container; it will just invoke the test binary.
	SelfManaged bool `yaml:"self_managed"`
	// Env is a map of environment variables to set in the container.
	Env map[string]string `yaml:"env"`
	// UseMount when set to true will create "mount" directory inside test folder and mount it into the container.
	UseMount bool `yaml:"use_mount"`
	// Ports are all the port mappings required by the test.
	Ports []PortMapping `yaml:"ports"`
	// Privileged if set to true will run the container in privileged mode.
	Privileged bool `yaml:"privileged"`
	// CapAdd is a list of kernel capabilities to add to the container.
	CapAdd []string `yaml:"cap_add"`
	// SecurityOpt is a list of string values to customize labels for MLS systems, such as SELinux.
	SecurityOpt []string `yaml:"security_opts"`
	// PIDMode is the PID namespace to use for the container (e.g. "host").
	PIDMode string `yaml:"pid_mode"`
	// TetragonImage is the Tetragon container image to run alongside the test to
	// record capability events. When empty, no Tetragon container is started.
	TetragonImage string `yaml:"tetragon_image"`
	// Capabilities, when non-nil, replaces the container's default Linux
	// capability set. The container is started with --cap-drop=ALL and then
	// --cap-add for each listed entry. An empty slice means the container gets
	// no Linux capabilities at all. When nil (the field is absent from
	// test.yaml) Docker's default capability set is used unchanged.
	Capabilities *[]string `yaml:"capabilities"`
}

// PortMapping maps a container port to a host port.
type PortMapping struct {
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol"`
	Host      int    `yaml:"host"`
}

// LoadTestConfig reads and parses a test.yaml file at the given path.
// Returns a zero-value TestConfig (not an error) when the file does not exist.
func LoadTestConfig(path string) (TestConfig, error) {
	var cfg TestConfig
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, yaml.Unmarshal(data, &cfg)
}
