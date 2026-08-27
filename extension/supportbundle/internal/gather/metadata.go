package gather

import (
	"context"
	"os"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// Metadata collects build and runtime information.
type Metadata struct{}

func (Metadata) Name() string { return "metadata" }

// metadata is the YAML document written to metadata.yaml.
type metadata struct {
	Command     string    `yaml:"command"`
	Description string    `yaml:"description"`
	Version     string    `yaml:"version"`
	GOOS        string    `yaml:"goos"`
	GOARCH      string    `yaml:"goarch"`
	NumCPU      int       `yaml:"num_cpu"`
	GOMAXPROCS  int       `yaml:"gomaxprocs"`
	GoVersion   string    `yaml:"go_version"`
	Uptime      string    `yaml:"uptime"`
	StartTime   time.Time `yaml:"start_time"`
	Hostname    string    `yaml:"hostname"`

	ResourceAttributes map[string]string `yaml:"resource_attributes,omitempty"`
}

func (Metadata) Gather(_ context.Context, opts Options) ([]File, error) {
	// A hostname error must not stop the bundle. Report it in the field instead.
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	m := metadata{
		Command:     opts.BuildInfo.Command,
		Description: opts.BuildInfo.Description,
		Version:     opts.BuildInfo.Version,
		GOOS:        runtime.GOOS,
		GOARCH:      runtime.GOARCH,
		NumCPU:      runtime.NumCPU(),
		GOMAXPROCS:  runtime.GOMAXPROCS(0),
		GoVersion:   runtime.Version(),
		Uptime:      time.Since(opts.StartTime).String(),
		StartTime:   opts.StartTime,
		Hostname:    hostname,

		ResourceAttributes: opts.ResourceAttributes,
	}

	content, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}

	return []File{{Path: "metadata.yaml", Content: content}}, nil
}
