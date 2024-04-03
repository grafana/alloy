package main

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// config holds configuration options to run the service.
type config struct {
	// ServicePath points to the path of the managed Alloy binary.
	ServicePath string

	// Args holds arguments to pass to the Alloy binary. os.Args[0] is not
	// included.
	Args []string

	// Environment holds environment variables for the Alloy service.
	// Each item represents an environment variable in form "key=value".
	// All environments variables from the current process with be merged into Environment
	Environment []string

	// WorkingDirectory points to the working directory to run the Alloy binary
	// from.
	WorkingDirectory string
}

// loadConfig loads the config from the Windows registry.
func loadConfig() (*config, error) {
	// NOTE(rfratto): the key name below shouldn't be changed without being
	// able to either migrate from the old key to the new key or supporting
	// both the old and the new key at the same time.

	alloyKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\GrafanaLabs\Alloy`, registry.READ)
	if err != nil {
		return nil, fmt.Errorf("failed to open registry: %w", err)
	}

	servicePath, _, err := alloyKey.GetStringValue("")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key (Default): %w", err)
	}

	args, _, err := alloyKey.GetStringsValue("Arguments")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key Arguments: %w", err)
	}

	env, _, err := alloyKey.GetStringsValue("Environment")
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key Environment: %w", err)
	}

	return &config{
		ServicePath:      servicePath,
		Args:             args,
		Environment:      env,
		WorkingDirectory: filepath.Dir(servicePath),
	}, nil
}
