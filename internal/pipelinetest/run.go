package pipelinetest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/pipelinetest/harness"
	"github.com/grafana/alloy/internal/statictest"
)

type TestConfig struct {
	DataPath string
	// BaseDir resolves a relative config path. RunTestFile sets it to the
	// test file's directory.
	BaseDir string
	// AssertTimeout bounds how long assertions are retried. Defaults to 30s.
	AssertTimeout time.Duration
	// LogWriter receives Alloy's logs. They are discarded when nil.
	LogWriter io.Writer
}

// Result is the outcome of a test that loaded and ran. Checks holds one entry
// per assertion, in the order of the test file.
type Result struct {
	Checks []harness.CheckResult
	// Snapshot is what reached the sink, as of the final assertion attempt.
	Snapshot harness.Snapshot
	// Unhealthy lists components that did not report healthy when the test
	// finished. It often explains why an assertion failed.
	Unhealthy []harness.ComponentHealth
	// Logs holds Alloy's first distinct warnings and errors.
	Logs []string
}

// Passed reports whether every check passed.
func (r Result) Passed() bool {
	for _, c := range r.Checks {
		if c.Err != nil {
			return false
		}
	}
	return true
}

const (
	// Kind is the value of the kind field in every test file.
	Kind = "pipelinetest"
	// Version is the test file format version this Alloy reads.
	Version = 1
)

// versions maps each test kind to the format version this Alloy reads.
var versions = map[string]int{
	Kind:            Version,
	statictest.Kind: statictest.Version,
}

// ErrNotTest means a file has no known kind.
var ErrNotTest = errors.New("not a test file")

// header holds the fields LoadTestFile checks before the full parse.
type header struct {
	Kind    string `yaml:"kind"`
	Version int    `yaml:"version"`
}

// readHeader returns the kind of a test file and checks its version. It
// parses only those two fields, so a file from a newer format reports its
// version, and not an unknown field. It returns the kind with a version
// error, so the caller knows which kind of test failed.
func readHeader(bb []byte) (string, error) {
	var h header
	if err := yaml.Unmarshal(bb, &h); err != nil {
		// A file that is not a YAML mapping is not a test.
		return "", fmt.Errorf("%w: %v", ErrNotTest, err)
	}
	want, ok := versions[h.Kind]
	if !ok {
		return "", fmt.Errorf("%w: kind is %q, not %s or %s", ErrNotTest, h.Kind, Kind, statictest.Kind)
	}
	if h.Version == 0 {
		return h.Kind, fmt.Errorf("version is missing, set version: %d", want)
	}
	if h.Version != want {
		return h.Kind, fmt.Errorf("%s version %d not supported, this Alloy supports version %d", h.Kind, h.Version, want)
	}
	return h.Kind, nil
}

// LoadTestFile reads a test file. It rejects a file without kind: pipelinetest
// or with an unsupported version. It also rejects unknown fields, so a typo
// fails the test.
func LoadTestFile(path string) (TestSchema, error) {
	bb, err := os.ReadFile(path)
	if err != nil {
		return TestSchema{}, err
	}
	if len(bytes.TrimSpace(bb)) == 0 {
		return TestSchema{}, errors.New("test file is empty")
	}
	kind, err := readHeader(bb)
	if err != nil {
		return TestSchema{}, err
	}
	if kind != Kind {
		return TestSchema{}, fmt.Errorf("kind is %s, not %s", kind, Kind)
	}

	var schema TestSchema
	dec := yaml.NewDecoder(bytes.NewReader(bb))
	dec.KnownFields(true)
	if err := dec.Decode(&schema); err != nil {
		if errors.Is(err, io.EOF) {
			return TestSchema{}, errors.New("test file is empty")
		}
		return TestSchema{}, err
	}
	return schema, nil
}

// Run runs one test and returns a Result with one entry per assertion. It
// returns an error when the test cannot run, for example when the config does
// not load or an input send fails.
func Run(ctx context.Context, schema TestSchema, cfg TestConfig) (Result, error) {
	if err := schema.Config.resolve(cfg.BaseDir); err != nil {
		return Result{}, err
	}
	if err := validateMocks(schema.Mocks); err != nil {
		return Result{}, err
	}
	if err := validateHTTPInputs(schema.Inputs.HTTP); err != nil {
		return Result{}, err
	}
	if err := validateSocketInputs(schema.Inputs.TCP, schema.Inputs.UDP); err != nil {
		return Result{}, err
	}

	checks, err := buildChecks(schema.Assert, schema.Config.Sinks)
	if err != nil {
		return Result{}, err
	}
	if len(checks) == 0 {
		return Result{}, errors.New("the test has no assertions")
	}

	built, err := buildSource(schema)
	if err != nil {
		return Result{}, err
	}

	configPath := schema.Config.Path
	if configPath == "" {
		configPath = "."
	}

	restoreEnv := setEnv(schema.Env)
	defer restoreEnv()

	alloy, err := harness.NewAlloy(harness.Config{
		SinkID:         "pipelinetest.sink.out",
		Sinks:          built.sinks,
		DataPath:       cfg.DataPath,
		Source:         built.source,
		SourceName:     configName(schema.Config),
		ConfigPath:     configPath,
		LokiPipelineV2: schema.Flags.LokiPipelineV2,
		AssertTimeout:  cfg.AssertTimeout,
		LogWriter:      cfg.LogWriter,
	})
	if err != nil {
		return Result{}, err
	}
	defer alloy.Stop()

	if err := produceInputs(ctx, alloy, built.inputs, schema.Inputs); err != nil {
		return Result{}, err
	}

	alloy.ObserveTargets(targetRefs(schema.Assert.Targets)...)
	results, snapshot := alloy.RunChecks(checks)
	return Result{
		Checks:    results,
		Snapshot:  snapshot,
		Unhealthy: alloy.UnhealthyComponents(),
		Logs:      alloy.ProblemLogs(),
	}, nil
}

// setEnv sets the test's environment variables, so sys.env in the config
// reads them, and returns a function that restores the previous values. Tests
// run one at a time, so changing the process environment is safe.
func setEnv(env map[string]string) func() {
	type prev struct {
		value string
		set   bool
	}
	saved := make(map[string]prev, len(env))
	for k, v := range env {
		old, ok := os.LookupEnv(k)
		saved[k] = prev{value: old, set: ok}
		_ = os.Setenv(k, v)
	}
	return func() {
		for k, p := range saved {
			if p.set {
				_ = os.Setenv(k, p.value)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}
}

// Err returns nil when every check passed. Otherwise it describes each failed
// assertion, Alloy's warnings and errors, and what reached the sink.
func (result Result) Err() error {
	if result.Passed() {
		return nil
	}
	var b strings.Builder
	b.WriteString("pipeline test failed\n\n")
	for _, c := range result.Checks {
		if c.Err != nil {
			fmt.Fprintf(&b, "- %s: %s\n", c.Name, c.Err)
		}
	}
	for _, u := range result.Unhealthy {
		fmt.Fprintf(&b, "\nunhealthy component %s (%s): %s", u.ID, u.Health, u.Message)
	}
	if len(result.Logs) > 0 {
		b.WriteString("\nalloy warnings and errors:\n")
		for _, l := range result.Logs {
			b.WriteString(l + "\n")
		}
	}
	b.WriteString("\nlatest snapshot:\n")
	b.WriteString(result.Snapshot.Text(50))
	return errors.New(b.String())
}

// RunTestFile loads and runs the test file at path. While the test runs, the
// working directory is the test file's directory. So config.path and the
// paths inside the config are relative to the test file.
func RunTestFile(ctx context.Context, path string, cfg TestConfig) (Result, error) {
	schema, err := LoadTestFile(path)
	if err != nil {
		return Result{}, err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Result{}, err
	}
	if cfg.DataPath, err = filepath.Abs(cfg.DataPath); err != nil {
		return Result{}, err
	}
	dir := filepath.Dir(abs)
	cfg.BaseDir = dir

	restore, err := chdir(dir)
	if err != nil {
		return Result{}, err
	}
	defer restore()
	return Run(ctx, schema, cfg)
}

// chdir changes the working directory and returns a function that restores
// it. Tests run one at a time, so changing it for the process is safe.
func chdir(dir string) (func(), error) {
	prev, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if err := os.Chdir(dir); err != nil {
		return nil, err
	}
	return func() { _ = os.Chdir(prev) }, nil
}
