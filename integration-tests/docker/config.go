package main

// TestConfig is used by tests to describe special setup requirements.
type TestConfig struct {
	Container            ContainerConfig             `yaml:"alloy_container"`
	AdditionalContainers []AdditionalContainerConfig `yaml:"additional_containers"`
}

// ContainerConfig is used to configure alloy container used for the test.
type ContainerConfig struct {
	// Dockerfile layers on the standard integration image; build context is the repository root.
	// Path is relative to the test directory.
	// When set, the runner builds and runs alloy-integration-tests-<test_dir>:latest.
	Dockerfile string `yaml:"dockerfile"`
	// UseMount when set to true will create "mount" directory inside test
	// folder that will be mounted into the container.
	UseMount bool `yaml:"use_mount"`
	// UseDockerSock when set to true will mount the host Docker socket into the container.
	UseDockerSock bool `yaml:"use_docker_sock"`
	// Ports are all the port mappings required by the test.
	// These will be configured and exposed for the container.
	Ports []PortMapping `yaml:"ports"`
	// Privileged if set to true will run alloy as a privileged container.
	Privileged bool `yaml:"privileged"`
	// CapAdd is a list of kernel capabilities to add to the container.
	CapAdd []string `yaml:"cap_add"`
	// SecurityOpt is a list of string values to customize labels for MLS systems, such as SELinux.
	SecurityOpt []string `yaml:"security_opts"`
	// PIDMode is the PID namespace to use for the container.
	PIDMode string `yaml:"pid_mode"`
}

// AdditionalContainerConfig is used to configure additional containers used for the test.
type AdditionalContainerConfig struct {
	// Name is the Docker container name.
	Name string `yaml:"name"`
	// Image is the Docker image to run or tag built from Build.
	Image string `yaml:"image"`
	// Build describes how to build the image for this container.
	Build AdditionalContainerBuildConfig `yaml:"build"`
	// Command overrides the default image command.
	Command []string `yaml:"command"`
}

// AdditionalContainerBuildConfig is used to build an additional container image.
type AdditionalContainerBuildConfig struct {
	// Context is the Docker build context directory.
	Context string `yaml:"context"`
	// Dockerfile is the Dockerfile path relative to Context.
	Dockerfile string `yaml:"dockerfile"`
}

type PortMapping struct {
	Container int    `yaml:"container"`
	Protocol  string `yaml:"protocol"`
	Host      int    `yaml:"host"`
}
