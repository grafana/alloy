//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"fmt"

	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/config"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
)

func (c *Component) writeConfigFile() (string, func(), error) {
	cfg := config.Build(c.args, config.Runtime{
		Port:       c.subprocess.Port(),
		HealthAddr: c.subprocess.HealthAddr(),
		OTLPAddr:   c.otlpReceiverAddr,
	}, c.opts.Logger)

	configData, err := yaml.Marshal(cfg)

	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	c.opts.Logger.Debug("generated Beyla YAML config", "yaml", string(configData))

	fd, err := unix.MemfdCreate("beyla-config", 0)

	if err != nil {
		return "", nil, fmt.Errorf("failed to create in-memory config file: %w", err)
	}

	if err := subprocess.WriteAll(fd, configData); err != nil {
		unix.Close(fd)
		return "", nil, fmt.Errorf("failed to write config: %w", err)
	}

	// seek to the beginning so the subprocess can read the config from the start.
	if _, err := unix.Seek(fd, 0, 0); err != nil {
		unix.Close(fd)
		return "", nil, fmt.Errorf("failed to seek config fd: %w", err)
	}

	configPath := fmt.Sprintf("/proc/self/fd/%d", fd)
	cleanup := func() { unix.Close(fd) }

	return configPath, cleanup, nil
}
