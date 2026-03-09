package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// TestConfig is used by tests to describe special setup requirements.
type TestConfig struct {
	Container ContainerConfig `yaml:"alloy_container"`
	Tetragon  TetragonConfig  `yaml:"tetragon_container"`
}

// ContainerConfig is used to configure alloy container used for the test.
type ContainerConfig struct {
	// User specifies the user (and optionally group) to run the container process as,
	// in "uid" or "uid:gid" format. When empty the image's default user is used (typically root).
	// Note: cap_add only adds to the bounding set for non-root users; without ambient capabilities
	// or file capabilities on the binary, a non-root process will have an empty effective set.
	// TODO: Find a way to grant capabilities to non-root users? There are two options:
	// 1. Ambient capabilities - set via PR_CAP_AMBIENT_RAISE at container startup
	// 2. File capabilities on the binary - RUN setcap CAP_SYS_PTRACE+eip /bin/alloy in the Dockerfile
	User string `yaml:"user"`
	// UseMount when set to true will create "mount" directory inside test
	// folder that will be mounted into the container.
	UseMount bool `yaml:"use_mount"`
	// Ports are all the port mappings required by the test.
	// These will be configured and exposed for the container.
	Ports []PortMapping `yaml:"ports"`
	// Privileged if set to true will run alloy as a privileged container.
	Privileged bool `yaml:"privileged"`
	// CapDrop is a list of kernel capabilities to drop from the container.
	// Use ["ALL"] to drop all default capabilities.
	CapDrop []string `yaml:"cap_drop"`
	// CapAdd is a list of kernel capabilities to add to the container.
	CapAdd []string `yaml:"cap_add"`
	// SecurityOpt is a list of string values to customize labels for MLS systems, such as SELinux.
	SecurityOpt []string `yaml:"security_opts"`
	// PIDMode is the PID namespace to use for the container (e.g. "host").
	PIDMode string `yaml:"pid_mode"`
}

// TetragonConfig configures the Tetragon sidecar container used to observe Linux capability events.
type TetragonConfig struct {
	// Image is the Tetragon container image (org/name:tag).
	// When Image is empty, no Tetragon container is started.
	Image string `yaml:"image"`
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
