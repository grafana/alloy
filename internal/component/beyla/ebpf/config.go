//go:build (linux && arm64) || (linux && amd64)

package beyla

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/config"
	"github.com/grafana/alloy/internal/component/beyla/ebpf/internal/subprocess"
)

func (c *Component) buildConfig(args Arguments) ([]byte, error) {
	cfg := config.Build(args, config.Runtime{
		Port:       c.subprocess.Port(),
		HealthAddr: c.subprocess.HealthAddr(),
		OTLPAddr:   c.otlpReceiverAddr,
	}, c.opts.Logger)

	configData, err := yaml.Marshal(cfg)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	// The config carries secrets, such as the peer metadata token, so log a redacted copy.
	if c.opts.Logger.Enabled(context.Background(), slog.LevelDebug) {
		if redacted, err := yaml.Marshal(config.Redacted(cfg)); err == nil {
			c.opts.Logger.Debug("generated Beyla YAML config", "yaml", string(redacted))
		}
	}

	return configData, nil
}

// configFileChildFD is the config file in the subprocess, after the config control socket.
const configFileChildFD = configControlChildFD + 1

// configFilePath is where the subprocess reads its config file.
func configFilePath() string {
	return fmt.Sprintf("/proc/self/fd/%d", configFileChildFD)
}

// writeConfigFile returns an in-memory file holding configData. It is close-on-exec, so only
// the subprocess inherits it. It's passed as one of its ExtraFiles.
func writeConfigFile(configData []byte) (*os.File, error) {
	fd, err := unix.MemfdCreate("beyla-config", unix.MFD_CLOEXEC)

	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory config file: %w", err)
	}

	if err := subprocess.WriteAll(fd, configData); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	// seek to the beginning so the subprocess can read the config from the start.
	if _, err := unix.Seek(fd, 0, 0); err != nil {
		unix.Close(fd)
		return nil, fmt.Errorf("failed to seek config fd: %w", err)
	}

	return os.NewFile(uintptr(fd), "beyla-config"), nil
}
