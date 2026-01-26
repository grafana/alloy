package main

// TestConfig is used by tests to describe special setup requirements.
type TestConfig struct {
	Container ContainerConfig `yaml:"alloy_container"`
}

// ContainerConfig is used to configure alloy container used for the test.
type ContainerConfig struct {
	// UseMount when set to true will create "mount" directory inside test
	// folder that will be mounted into the countainer.
	UseMount bool `yaml:"use_mount"`
	// Ports are all the port mappings required by the test.
	// These will be configured and exposed for the container.
	Ports []PortMapping `yaml:"ports"`
	// Privileged if set to true will run alloy as a privileged container.
	Privileged bool `yaml:"privileged"`
	// List of kernel capabilities to add to the container
	CapAdd []string `yaml:"cap_add"`
	// List of string values to customize labels for MLS systems, such as SELinux.
	SecurityOpt []string `yaml:"security_opts"`
	// PID namespace to use for the container
	PIDMode string `yaml:"pid_mode"`
}

type PortMapping struct {
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol"`
	Host      int    `yaml:"host"`
}
