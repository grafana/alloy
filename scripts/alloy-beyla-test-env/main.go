// Alloy + Beyla Docker test environment.
//
// Creates a Docker-based test environment for Alloy with beyla.ebpf,
// prometheus.exporter.unix, and prometheus.echo (metrics to stdout).
// The Alloy config is stored on the host and mounted into the container.
//
// Usage:
//
//	go run . [CONFIG_DIR]
//	go run . --config-dir=/path/to/config [FLAGS]
//
// Environment variables (optional overrides): ALLOY_VERSION, CONTAINER_NAME, IMAGE.
package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/grafana/alloy/scripts/alloy-beyla-test-env/internal/docker"
	"github.com/spf13/cobra"
)

//go:embed config.alloy
var defaultConfig string

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var (
	configDir     string
	aloyVersion   string
	containerName string
	image         string
)

var rootCmd = &cobra.Command{
	Use:   "alloy-beyla-test-env [CONFIG_DIR]",
	Short: "Run Alloy with beyla.ebpf and prometheus.exporter.unix in Docker",
	Long:  "Starts a privileged container that installs Alloy and runs it with the given config directory mounted at /etc/alloy. Metrics are echoed to stdout.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  run,
}

func init() {
	wd, _ := os.Getwd()
	defaultConfigDir := filepath.Join(wd, "alloy-beyla-config")

	rootCmd.Flags().StringVarP(&configDir, "config-dir", "c", defaultConfigDir, "Host directory for Alloy config (mounted at /etc/alloy)")
	rootCmd.Flags().StringVar(&aloyVersion, "alloy-version", getEnv("ALLOY_VERSION", "1.13.2"), "Alloy version to install")
	rootCmd.Flags().StringVar(&containerName, "container-name", getEnv("CONTAINER_NAME", "alloy-beyla-test"), "Docker container name")
	rootCmd.Flags().StringVar(&image, "image", getEnv("IMAGE", "ubuntu:22.04"), "Base Docker image")
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		configDir = args[0]
	}

	absConfigDir, err := filepath.Abs(configDir)
	if err != nil {
		absConfigDir = configDir
	}

	if err := os.MkdirAll(absConfigDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", absConfigDir, err)
	}

	configPath := filepath.Join(absConfigDir, "config.alloy")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", configPath, err)
		}
		fmt.Printf("Creating default config at %s\n", configPath)
	}

	fmt.Printf("Using config directory: %s\n", absConfigDir)
	fmt.Printf("Alloy config file:     %s\n", configPath)
	fmt.Printf("Container name:        %s\n\n", containerName)

	return docker.Run(docker.Options{
		ContainerName: containerName,
		Image:         image,
		ConfigDir:     absConfigDir,
		AlloyVersion:  aloyVersion,
	})
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
