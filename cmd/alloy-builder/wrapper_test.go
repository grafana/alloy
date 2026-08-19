package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component/all/catalog"
)

const alloyEngineExtensionYAML = `extensions:
  - gomod: github.com/grafana/alloy v1.18.0
    import: github.com/grafana/alloy/extension/alloyengine
    name: alloyengine
`

type fakeDelegate struct {
	calls      int
	invocation delegateInvocation
	exitCode   int
	err        error
	inspect    func(delegateInvocation)
}

func (f *fakeDelegate) Run(invocation delegateInvocation) (int, error) {
	f.calls++
	f.invocation = invocation
	f.invocation.Args = append([]string(nil), invocation.Args...)
	f.invocation.Env = append([]string(nil), invocation.Env...)
	if f.inspect != nil {
		f.inspect(invocation)
	}
	return f.exitCode, f.err
}

func TestDefaultAndExplicitDelegate(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
	}{
		{
			name:     "pinned default",
			args:     []string{"--verbose"},
			wantName: "go",
			wantArgs: []string{"run", ocbModule + "@" + defaultOCBVersion, "--verbose"},
		},
		{
			name:     "version override",
			args:     []string{"--ocb-version", "v0.140.1", "--verbose"},
			wantName: "go",
			wantArgs: []string{"run", ocbModule + "@v0.140.1", "--verbose"},
		},
		{
			name:     "executable override",
			args:     []string{"--ocb-version=v0.140.1", "--ocb=/tmp/fake-ocb", "--verbose"},
			wantName: "/tmp/fake-ocb",
			wantArgs: []string{"--verbose"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeDelegate{}
			if got := run(test.args, strings.NewReader(""), io.Discard, io.Discard, []string{"A=B"}, fake); got != 0 {
				t.Fatalf("run() = %d, want 0", got)
			}
			if fake.invocation.Name != test.wantName {
				t.Fatalf("delegate name = %q, want %q", fake.invocation.Name, test.wantName)
			}
			if !reflect.DeepEqual(fake.invocation.Args, test.wantArgs) {
				t.Fatalf("delegate args = %#v, want %#v", fake.invocation.Args, test.wantArgs)
			}
		})
	}
}

func TestManifestWithoutAlloyComponentsPassesThrough(t *testing.T) {
	manifest := writeManifest(t, `# OCB must diagnose these itself when the Alloy field is absent.
unknown_future_key: true
dist:
  build_tags: from_file
`)
	stdin := strings.NewReader("input")
	var stdout, stderr bytes.Buffer
	environ := []string{"A=B", "dist.build_tags=from_environment"}
	fake := &fakeDelegate{exitCode: 37}
	fake.inspect = func(invocation delegateInvocation) {
		if invocation.Stdin != stdin || invocation.Stdout != &stdout || invocation.Stderr != &stderr {
			t.Error("standard streams were not forwarded to the delegate")
		}
		_, _ = io.WriteString(invocation.Stdout, "delegate stdout")
		_, _ = io.WriteString(invocation.Stderr, "delegate stderr")
	}

	code := run([]string{"--ocb", "fake-ocb", "--config=" + manifest, "--skip-compilation"}, stdin, &stdout, &stderr, environ, fake)
	if code != 37 {
		t.Fatalf("run() = %d, want delegated exit code 37", code)
	}
	if fake.invocation.Name != "fake-ocb" {
		t.Fatalf("delegate name = %q", fake.invocation.Name)
	}
	if !reflect.DeepEqual(fake.invocation.Args, []string{"--config=" + manifest, "--skip-compilation"}) {
		t.Fatalf("delegate args changed: %#v", fake.invocation.Args)
	}
	if !reflect.DeepEqual(fake.invocation.Env, environ) {
		t.Fatalf("delegate environment changed: %#v", fake.invocation.Env)
	}
	if stdout.String() != "delegate stdout" || stderr.String() != "delegate stderr" {
		t.Fatalf("delegate streams were not preserved: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestEmptyManifestPassesThrough(t *testing.T) {
	manifest := writeManifest(t, "")
	fake := &fakeDelegate{}
	args := []string{"--ocb=fake", "--config", manifest}
	if code := run(args, nil, io.Discard, io.Discard, []string{"A=B"}, fake); code != 0 {
		t.Fatalf("run() = %d", code)
	}
	if fake.calls != 1 || !reflect.DeepEqual(fake.invocation.Args, []string{"--config", manifest}) {
		t.Fatalf("empty manifest was not delegated unchanged: calls=%d args=%#v", fake.calls, fake.invocation.Args)
	}
}

func TestConfigArgumentFormsRewritePrivateTemporaryManifest(t *testing.T) {
	tests := []struct {
		name string
		args func(string) []string
	}{
		{name: "long separate", args: func(path string) []string { return []string{"--config", path} }},
		{name: "long inline", args: func(path string) []string { return []string{"--config=" + path} }},
		{name: "short separate", args: func(path string) []string { return []string{"-c", path} }},
		{name: "short equals", args: func(path string) []string { return []string{"-c=" + path} }},
		{name: "short compact", args: func(path string) []string { return []string{"-c" + path} }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := writeManifest(t, `# keep this comment
dist:
  build_tags: gore2regex nodocker gore2regex
`+alloyEngineExtensionYAML+`alloy_components:
  - prometheus.scrape
  - loki.write
`)
			var rewritten []byte
			var temporaryPath string
			fake := &fakeDelegate{inspect: func(invocation delegateInvocation) {
				temporaryPath = delegatedConfigPath(t, invocation.Args)
				if temporaryPath == manifest {
					t.Fatal("delegate received the source manifest instead of a rewritten manifest")
				}
				if filepath.Dir(temporaryPath) == filepath.Dir(manifest) {
					t.Fatalf("temporary manifest was written beside the source manifest: %q", temporaryPath)
				}
				info, err := os.Stat(temporaryPath)
				if err != nil {
					t.Fatalf("stat temporary manifest during delegation: %v", err)
				}
				if permission := info.Mode().Perm(); runtime.GOOS != "windows" && permission != 0o600 {
					t.Fatalf("temporary manifest permissions = %o, want 600", permission)
				}
				rewritten, err = os.ReadFile(temporaryPath)
				if err != nil {
					t.Fatalf("read temporary manifest during delegation: %v", err)
				}
			}}
			args := append([]string{"--ocb=fake-ocb"}, test.args(manifest)...)
			args = append(args, "--verbose")
			var stderr bytes.Buffer
			if got := run(args, strings.NewReader(""), io.Discard, &stderr, []string{"A=B"}, fake); got != 0 {
				t.Fatalf("run() = %d, stderr: %s", got, stderr.String())
			}
			if _, err := os.Stat(temporaryPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("temporary manifest was not removed: %v", err)
			}
			if slices.Contains(fake.invocation.Args, "-c") {
				t.Fatalf("short config flag was not normalized for stock OCB: %#v", fake.invocation.Args)
			}
			assertRewrittenManifest(t, rewritten, "gore2regex,nodocker,alloy_custom_components,alloy_component_loki_write,alloy_component_prometheus_scrape")
			if !bytes.Contains(rewritten, []byte("# keep this comment")) {
				t.Fatal("rewritten manifest did not preserve its YAML comment")
			}
			if !strings.Contains(stderr.String(), "effective build tags:") {
				t.Fatalf("effective build tags were not reported: %q", stderr.String())
			}
		})
	}
}

func TestManifestInReadOnlyDirectoryCanBeRewritten(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on Windows")
	}

	directory := filepath.Join(t.TempDir(), "read-only")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	manifest := filepath.Join(directory, "builder-config.yaml")
	contents := alloyEngineExtensionYAML + "alloy_components: [loki.write]\n"
	if err := os.WriteFile(manifest, []byte(contents), 0o600); err != nil {
		t.Fatalf("write source manifest: %v", err)
	}
	if err := os.Chmod(directory, 0o555); err != nil {
		t.Fatalf("make source directory read-only: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(directory, 0o755); err != nil {
			t.Errorf("restore source directory permissions: %v", err)
		}
	})

	var temporaryPath string
	fake := &fakeDelegate{inspect: func(invocation delegateInvocation) {
		temporaryPath = delegatedConfigPath(t, invocation.Args)
		if filepath.Dir(temporaryPath) == directory {
			t.Fatalf("temporary manifest was written in read-only source directory: %q", temporaryPath)
		}
		if _, err := os.ReadFile(temporaryPath); err != nil {
			t.Fatalf("read delegated temporary manifest: %v", err)
		}
	}}
	var stderr bytes.Buffer
	code := run([]string{"--ocb=fake", "--config", manifest}, nil, io.Discard, &stderr, nil, fake)
	if code != 0 || fake.calls != 1 {
		t.Fatalf("run()=%d delegate calls=%d, stderr=%s", code, fake.calls, stderr.String())
	}
	if _, err := os.Stat(temporaryPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary manifest was not removed: %v", err)
	}
}

func TestEnvironmentBuildTagsOverrideIsMergedAndRemoved(t *testing.T) {
	manifest := writeManifest(t, `dist:
  build_tags: ignored_file_tag
`+alloyEngineExtensionYAML+`alloy_components:
  - loki.write
`)
	environ := []string{
		"A=B",
		"dist.build_tags=environment_tag environment_tag,second_tag",
		"DIST_BUILD_TAGS=untouched",
	}
	var rewritten []byte
	fake := &fakeDelegate{inspect: func(invocation delegateInvocation) {
		var err error
		rewritten, err = os.ReadFile(delegatedConfigPath(t, invocation.Args))
		if err != nil {
			t.Fatalf("read temporary manifest: %v", err)
		}
		wantEnv := []string{"A=B", "DIST_BUILD_TAGS=untouched"}
		if !reflect.DeepEqual(invocation.Env, wantEnv) {
			t.Fatalf("delegate environment = %#v, want %#v", invocation.Env, wantEnv)
		}
	}}
	if got := run([]string{"--ocb=fake", "--config", manifest}, nil, io.Discard, io.Discard, environ, fake); got != 0 {
		t.Fatalf("run() = %d", got)
	}
	assertRewrittenManifest(t, rewritten, "environment_tag,second_tag,alloy_custom_components,alloy_component_loki_write")
}

func TestEmptyAlloyComponentsSelectsNoNativeComponents(t *testing.T) {
	manifest := writeManifest(t, alloyEngineExtensionYAML+`alloy_components: []
`)
	var rewritten []byte
	fake := &fakeDelegate{inspect: func(invocation delegateInvocation) {
		var err error
		rewritten, err = os.ReadFile(delegatedConfigPath(t, invocation.Args))
		if err != nil {
			t.Fatalf("read temporary manifest: %v", err)
		}
	}}
	if got := run([]string{"--ocb=fake", "--config", manifest}, nil, io.Discard, io.Discard, nil, fake); got != 0 {
		t.Fatalf("run() = %d", got)
	}
	assertRewrittenManifest(t, rewritten, catalog.CustomBuildTag)
}

func TestManifestValidationErrorsDoNotDelegate(t *testing.T) {
	validSelection := `alloy_components:
  - loki.write
`
	tests := []struct {
		name        string
		manifest    string
		environ     []string
		wantMessage string
	}{
		{name: "null selection", manifest: alloyEngineExtensionYAML + "alloy_components: null\n", wantMessage: "must be a YAML list"},
		{name: "scalar selection", manifest: alloyEngineExtensionYAML + "alloy_components: loki.write\n", wantMessage: "must be a YAML list"},
		{name: "mapping selection", manifest: alloyEngineExtensionYAML + "alloy_components: {component: loki.write}\n", wantMessage: "must be a YAML list"},
		{name: "non-string item", manifest: alloyEngineExtensionYAML + "alloy_components: [123]\n", wantMessage: "item 0 must be a non-empty string"},
		{name: "empty item", manifest: alloyEngineExtensionYAML + "alloy_components: [\"\"]\n", wantMessage: "item 0 must be a non-empty string"},
		{name: "unknown component", manifest: alloyEngineExtensionYAML + "alloy_components: [not.real]\n", wantMessage: "unknown Alloy component"},
		{name: "duplicate component", manifest: alloyEngineExtensionYAML + "alloy_components: [loki.write, loki.write]\n", wantMessage: "duplicate Alloy component"},
		{name: "missing extension", manifest: validSelection, wantMessage: "requires an extensions list"},
		{name: "wrong extension", manifest: "extensions: [{name: alloyengine, import: example.com/not-alloy}]\n" + validSelection, wantMessage: "requires the alloyengine extension import"},
		{name: "null dist", manifest: "dist: null\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "dist must be a mapping"},
		{name: "null build tags", manifest: "dist: {build_tags: null}\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "dist.build_tags must be a string"},
		{name: "reserved marker", manifest: "dist: {build_tags: alloy_custom_components}\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "reserved Alloy build tag"},
		{name: "reserved component tag", manifest: "dist: {build_tags: alloy_component_loki_write}\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "reserved Alloy build tag"},
		{name: "reserved environment tag", manifest: alloyEngineExtensionYAML + validSelection, environ: []string{"dist.build_tags=alloy_custom_components"}, wantMessage: "reserved Alloy build tag"},
		{name: "unknown top-level key", manifest: "future_key: []\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "unknown top-level key \"future_key\""},
		{name: "duplicate top-level key", manifest: alloyEngineExtensionYAML + validSelection + "alloy_components: []\n", wantMessage: "duplicate mapping key \"alloy_components\""},
		{name: "duplicate nested key", manifest: "dist:\n  build_tags: one\n  build_tags: two\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "duplicate mapping key \"build_tags\""},
		{name: "alias", manifest: "receivers: &shared []\nprocessors: *shared\n" + alloyEngineExtensionYAML + validSelection, wantMessage: "YAML aliases are not supported"},
		{name: "multiple documents", manifest: alloyEngineExtensionYAML + validSelection + "---\ndist: {}\n", wantMessage: "multiple YAML documents are not supported"},
		{name: "malformed YAML", manifest: "alloy_components: [loki.write\n", wantMessage: "parse OCB manifest"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := writeManifest(t, test.manifest)
			fake := &fakeDelegate{}
			var stderr bytes.Buffer
			code := run([]string{"--ocb=fake", "--config", manifest}, nil, io.Discard, &stderr, test.environ, fake)
			if code == 0 {
				t.Fatalf("run() succeeded; stderr: %s", stderr.String())
			}
			if fake.calls != 0 {
				t.Fatalf("delegate was called %d times", fake.calls)
			}
			if !strings.Contains(stderr.String(), test.wantMessage) {
				t.Fatalf("stderr %q does not contain %q", stderr.String(), test.wantMessage)
			}
		})
	}
}

func TestAlloyComponentsRejectsSkipCompilation(t *testing.T) {
	manifest := writeManifest(t, alloyEngineExtensionYAML+`alloy_components: []
`)
	tests := []struct {
		name         string
		flags        []string
		wantDelegate bool
	}{
		{name: "enabled", flags: []string{"--skip-compilation"}},
		{name: "enabled explicitly", flags: []string{"--skip-compilation=true"}},
		{name: "disabled", flags: []string{"--skip-compilation=false"}, wantDelegate: true},
		{name: "last value wins", flags: []string{"--skip-compilation", "--skip-compilation=false"}, wantDelegate: true},
		{name: "invalid value left to OCB", flags: []string{"--skip-compilation=invalid"}, wantDelegate: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeDelegate{}
			var stderr bytes.Buffer
			args := []string{"--ocb=fake", "--config", manifest}
			args = append(args, test.flags...)
			code := run(args, nil, io.Discard, &stderr, nil, fake)
			if test.wantDelegate {
				if code != 0 || fake.calls != 1 {
					t.Fatalf("run()=%d delegate calls=%d, want successful delegation; stderr=%s", code, fake.calls, stderr.String())
				}
				return
			}
			if code == 0 || fake.calls != 0 {
				t.Fatalf("run()=%d delegate calls=%d, want pre-delegation failure", code, fake.calls)
			}
			if !strings.Contains(stderr.String(), "cannot be used with --skip-compilation") {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
	}
}

func TestLastConfigFlagIsEffective(t *testing.T) {
	manifest := writeManifest(t, alloyEngineExtensionYAML+`alloy_components: []
`)
	fake := &fakeDelegate{}
	code := run([]string{"--ocb=fake", "--config", filepath.Join(t.TempDir(), "does-not-exist.yaml"), "--config", manifest}, nil, io.Discard, io.Discard, nil, fake)
	if code != 0 || fake.calls != 1 {
		t.Fatalf("run()=%d delegate calls=%d", code, fake.calls)
	}
}

func TestArgumentErrorsAndStockFlagValues(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantMessage string
	}{
		{name: "ocb missing", args: []string{"--ocb"}, wantMessage: "requires an executable path"},
		{name: "ocb empty", args: []string{"--ocb="}, wantMessage: "non-empty executable path"},
		{name: "version missing", args: []string{"--ocb-version"}, wantMessage: "requires a version"},
		{name: "version empty", args: []string{"--ocb-version="}, wantMessage: "non-empty version"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fake := &fakeDelegate{}
			var stderr bytes.Buffer
			if code := run(test.args, nil, io.Discard, &stderr, nil, fake); code == 0 {
				t.Fatal("run() succeeded")
			}
			if fake.calls != 0 || !strings.Contains(stderr.String(), test.wantMessage) {
				t.Fatalf("calls=%d stderr=%q", fake.calls, stderr.String())
			}
		})
	}

	plan, err := parseArguments([]string{"--ldflags", "--ocb", "--", "--ocb-version", "future"})
	if err != nil {
		t.Fatalf("parseArguments() error: %v", err)
	}
	want := []string{"--ldflags", "--ocb", "--", "--ocb-version", "future"}
	if plan.ocbPath != "" || plan.ocbVersion != defaultOCBVersion || !reflect.DeepEqual(plan.delegateArgs, want) {
		t.Fatalf("stock value or arguments after -- were consumed: %#v", plan)
	}
	value, valid := booleanFlagValue([]string{"--ldflags", "--skip-compilation", "--skip-compilation=false"}, "--skip-compilation")
	if !valid || value {
		t.Fatalf("boolean flag parser treated a stock string value as a flag: value=%t valid=%t", value, valid)
	}
}

func TestDelegateFailureAndExitCode(t *testing.T) {
	t.Run("startup error", func(t *testing.T) {
		fake := &fakeDelegate{err: errors.New("not executable")}
		var stderr bytes.Buffer
		if code := run([]string{"--ocb=fake"}, nil, io.Discard, &stderr, nil, fake); code != 1 {
			t.Fatalf("run() = %d, want 1", code)
		}
		if !strings.Contains(stderr.String(), "start OCB delegate: not executable") {
			t.Fatalf("stderr = %q", stderr.String())
		}
	})

	t.Run("exit code", func(t *testing.T) {
		fake := &fakeDelegate{exitCode: 19}
		if code := run([]string{"--ocb=fake"}, nil, io.Discard, io.Discard, nil, fake); code != 19 {
			t.Fatalf("run() = %d, want 19", code)
		}
	})
}

func writeManifest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "builder-config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func delegatedConfigPath(t *testing.T, args []string) string {
	t.Helper()
	var result string
	for i, arg := range args {
		switch {
		case arg == "--config" && i+1 < len(args):
			result = args[i+1]
		case strings.HasPrefix(arg, "--config="):
			result = strings.TrimPrefix(arg, "--config=")
		}
	}
	if result == "" {
		t.Fatalf("delegate args do not contain a config path: %#v", args)
	}
	return result
}

func assertRewrittenManifest(t *testing.T, data []byte, wantBuildTags string) {
	t.Helper()
	document, err := decodeYAMLDocument(data)
	if err != nil {
		t.Fatalf("parse rewritten manifest: %v\n%s", err, data)
	}
	root := documentMapping(document)
	if root == nil {
		t.Fatalf("rewritten manifest is not a mapping:\n%s", data)
	}
	if mappingHasKey(root, "alloy_components") {
		t.Fatalf("rewritten manifest still contains alloy_components:\n%s", data)
	}
	gotBuildTags, err := manifestBuildTags(root)
	if err != nil {
		t.Fatalf("read rewritten build tags: %v", err)
	}
	if gotBuildTags != wantBuildTags {
		t.Fatalf("build tags = %q, want %q\n%s", gotBuildTags, wantBuildTags, data)
	}
}

func TestBuildTagSplitAndDeduplication(t *testing.T) {
	got, err := validateUnrelatedBuildTags(" zeta,alpha  zeta\tbeta,alpha ")
	if err != nil {
		t.Fatalf("validateUnrelatedBuildTags() error: %v", err)
	}
	want := []string{"zeta", "alpha", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tags = %s, want %s", fmt.Sprint(got), fmt.Sprint(want))
	}
}
