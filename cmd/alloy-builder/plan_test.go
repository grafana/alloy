package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component/all/catalog"
	"gopkg.in/yaml.v3"
)

func TestParseBuildPlanArguments(t *testing.T) {
	options, command, err := parseBuildPlanArguments([]string{
		"--config", "custom.yaml",
		"--default-config=default.yaml",
		"--repo-root", "/repo",
		"--build-tags", "one two",
		"--skip-generation",
		"--", "make", "target", "FLAG=value",
	})
	if err != nil {
		t.Fatalf("parseBuildPlanArguments() error: %v", err)
	}
	if options.configPath != "custom.yaml" || options.defaultConfig != "default.yaml" || options.repoRoot != "/repo" {
		t.Fatalf("unexpected paths: %#v", options)
	}
	if options.buildTags != "one two" || !options.skipGeneration {
		t.Fatalf("unexpected build options: %#v", options)
	}
	if !reflect.DeepEqual(command, []string{"make", "target", "FLAG=value"}) {
		t.Fatalf("command = %#v", command)
	}

	for _, args := range [][]string{{}, {"--config", "config.yaml"}, {"--"}} {
		if _, _, err := parseBuildPlanArguments(args); err == nil {
			t.Fatalf("parseBuildPlanArguments(%#v) succeeded without a child command", args)
		}
	}
}

func TestPrepareInTreeBuildPlanUsesImplicitFullManifest(t *testing.T) {
	repoRoot := t.TempDir()
	manifest := writeBuildPlanManifest(t, repoRoot, `dist:
  module: example.com/ignored
  output_path: ./ignored
  build_tags: ignored_file_tag
replaces:
  - github.com/grafana/alloy => ../ignored
  - github.com/grafana/alloy/syntax => ../ignored/syntax
  - example.com/old => example.com/new v1.2.3
`)
	plan, err := prepareInTreeBuildPlan(buildPlanOptions{
		configPath:    manifest,
		defaultConfig: manifest,
		repoRoot:      repoRoot,
		buildTags:     "caller_tag caller_tag,second_tag",
	}, []string{"A=B", buildTagsEnv + "=environment_tag"})
	if err != nil {
		t.Fatalf("prepareInTreeBuildPlan() error: %v", err)
	}
	t.Cleanup(func() {
		if plan.cleanup != nil {
			_ = plan.cleanup()
		}
	})

	if plan.selection.mode != "full" || plan.selection.source != "alloy_components omitted" || !plan.needsBeyla {
		t.Fatalf("unexpected selection: %#v, needsBeyla=%t", plan.selection, plan.needsBeyla)
	}
	if want := []string{"environment_tag", "caller_tag", "second_tag"}; !reflect.DeepEqual(plan.buildTags, want) {
		t.Fatalf("build tags = %#v, want %#v", plan.buildTags, want)
	}
	if !reflect.DeepEqual(plan.environ, []string{"A=B"}) {
		t.Fatalf("child environment = %#v", plan.environ)
	}
	if filepath.Dir(plan.outputPath) != filepath.Dir(plan.configPath) || plan.outputPath == filepath.Join(repoRoot, "collector") {
		t.Fatalf("plan did not use one private workspace: config=%q output=%q", plan.configPath, plan.outputPath)
	}

	data, err := os.ReadFile(plan.configPath)
	if err != nil {
		t.Fatalf("read rewritten manifest: %v", err)
	}
	document, err := decodeYAMLDocument(data)
	if err != nil {
		t.Fatalf("decode rewritten manifest: %v", err)
	}
	root := documentMapping(document)
	distribution, _ := mappingValue(root, "dist")
	assertMappingString(t, distribution, "module", inTreeModule)
	assertMappingString(t, distribution, "output_path", plan.outputPath)
	assertMappingString(t, distribution, "build_tags", "environment_tag,caller_tag,second_tag")
	assertReplacement(t, root, "github.com/grafana/alloy", filepath.ToSlash(repoRoot))
	assertReplacement(t, root, "github.com/grafana/alloy/syntax", filepath.ToSlash(filepath.Join(repoRoot, "syntax")))
	assertReplacement(t, root, "example.com/old", "example.com/new v1.2.3")
}

func TestPrepareInTreeBuildPlanUsesManifestNativeSelection(t *testing.T) {
	repoRoot := t.TempDir()
	manifest := writeBuildPlanManifest(t, repoRoot, `dist:
  build_tags: fixture_tag
alloy_components:
  - remote.http
  - loki.write
`)
	plan, err := prepareInTreeBuildPlan(buildPlanOptions{
		configPath:    manifest,
		defaultConfig: manifest,
		repoRoot:      repoRoot,
		buildTags:     "gore2regex",
	}, nil)
	if err != nil {
		t.Fatalf("prepareInTreeBuildPlan() error: %v", err)
	}
	defer func() { _ = plan.cleanup() }()

	wantComponents := []string{"loki.write", "remote.http"}
	if plan.selection.mode != "custom" || plan.selection.source != "builder manifest" || !reflect.DeepEqual(plan.selection.components, wantComponents) {
		t.Fatalf("selection = %#v", plan.selection)
	}
	if plan.needsBeyla {
		t.Fatal("minimal manifest unexpectedly requires Beyla assets")
	}
	wantTags := []string{
		"fixture_tag",
		"gore2regex",
		catalog.CustomBuildTag,
		"alloy_component_loki_write",
		"alloy_component_remote_http",
	}
	if !reflect.DeepEqual(plan.buildTags, wantTags) {
		t.Fatalf("build tags = %#v, want %#v", plan.buildTags, wantTags)
	}
	data, err := os.ReadFile(plan.configPath)
	if err != nil {
		t.Fatalf("read rewritten manifest: %v", err)
	}
	if bytes.Contains(data, []byte("alloy_components:")) {
		t.Fatalf("rewritten manifest still contains alloy_components:\n%s", data)
	}
}

func TestPrepareInTreeBuildPlanMergesDefaultDependencyPolicy(t *testing.T) {
	repoRoot := t.TempDir()
	customManifest := writeBuildPlanManifest(t, repoRoot, `replaces:
  - example.com/custom => example.com/fork v1.0.0
excludes:
  - example.com/custom v0.9.0
alloy_components: []
`)
	defaultManifest := filepath.Join(repoRoot, "default-builder-config.yaml")
	if err := os.WriteFile(defaultManifest, []byte(`replaces:
  - example.com/default => example.com/fork v2.0.0
  - example.com/custom => example.com/ignored v9.0.0
excludes:
  - example.com/default v1.0.0
`), 0o600); err != nil {
		t.Fatalf("write default manifest: %v", err)
	}

	plan, err := prepareInTreeBuildPlan(buildPlanOptions{
		configPath:    customManifest,
		defaultConfig: defaultManifest,
		repoRoot:      repoRoot,
	}, nil)
	if err != nil {
		t.Fatalf("prepareInTreeBuildPlan() error: %v", err)
	}
	defer func() { _ = plan.cleanup() }()
	data, err := os.ReadFile(plan.configPath)
	if err != nil {
		t.Fatalf("read rewritten manifest: %v", err)
	}
	document, err := decodeYAMLDocument(data)
	if err != nil {
		t.Fatalf("decode rewritten manifest: %v", err)
	}
	root := documentMapping(document)
	assertReplacement(t, root, "example.com/custom", "example.com/fork v1.0.0")
	assertReplacement(t, root, "example.com/default", "example.com/fork v2.0.0")
	excludes, found := mappingValue(root, "excludes")
	if !found || len(excludes.Content) != 2 {
		t.Fatalf("merged excludes = %#v", excludes)
	}
}

func TestPrepareInTreeBuildPlanRebasesRelativeLocalReplacements(t *testing.T) {
	repoRoot := t.TempDir()
	manifest := writeBuildPlanManifest(t, repoRoot, `dist:
  output_path: ./original-output
replaces:
  - example.com/local => ../local-module
  - example.com/remote => example.com/fork v1.2.3
`)
	plan, err := prepareInTreeBuildPlan(buildPlanOptions{
		configPath:    manifest,
		defaultConfig: manifest,
		repoRoot:      repoRoot,
	}, nil)
	if err != nil {
		t.Fatalf("prepareInTreeBuildPlan() error: %v", err)
	}
	defer func() { _ = plan.cleanup() }()

	data, err := os.ReadFile(plan.configPath)
	if err != nil {
		t.Fatalf("read rewritten manifest: %v", err)
	}
	document, err := decodeYAMLDocument(data)
	if err != nil {
		t.Fatalf("decode rewritten manifest: %v", err)
	}
	root := documentMapping(document)
	assertReplacement(t, root, "example.com/local", filepath.ToSlash(filepath.Join(repoRoot, "local-module")))
	assertReplacement(t, root, "example.com/remote", "example.com/fork v1.2.3")
}

func TestPrepareInTreeBuildPlanSkipGenerationUsesCheckedInCollector(t *testing.T) {
	repoRoot := t.TempDir()
	manifest := writeBuildPlanManifest(t, repoRoot, "alloy_components: []\n")
	plan, err := prepareInTreeBuildPlan(buildPlanOptions{
		configPath:     manifest,
		defaultConfig:  manifest,
		repoRoot:       repoRoot,
		buildTags:      "gore2regex",
		skipGeneration: true,
	}, nil)
	if err != nil {
		t.Fatalf("prepareInTreeBuildPlan() error: %v", err)
	}
	if plan.cleanup != nil {
		t.Fatal("skip-generation plan unexpectedly allocated a temporary workspace")
	}
	if plan.configPath != manifest || plan.outputPath != filepath.Join(repoRoot, "collector") {
		t.Fatalf("config=%q output=%q", plan.configPath, plan.outputPath)
	}
	if want := []string{"gore2regex", catalog.CustomBuildTag}; !reflect.DeepEqual(plan.buildTags, want) {
		t.Fatalf("build tags = %#v, want %#v", plan.buildTags, want)
	}
}

func TestPrepareInTreeBuildPlanRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name        string
		manifest    string
		options     buildPlanOptions
		wantMessage string
	}{
		{
			name:        "reserved marker tag",
			manifest:    "alloy_components: [remote.http]\n",
			options:     buildPlanOptions{buildTags: "alloy_custom_components"},
			wantMessage: "reserved Alloy build tag",
		},
		{
			name:        "reserved component tag",
			manifest:    "dist: {}\n",
			options:     buildPlanOptions{buildTags: "alloy_component_remote_http"},
			wantMessage: "configure native components with alloy_components",
		},
		{
			name:        "invalid output path",
			manifest:    "dist: {output_path: []}\n",
			wantMessage: "dist.output_path must be a string",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repoRoot := t.TempDir()
			manifest := writeBuildPlanManifest(t, repoRoot, test.manifest)
			test.options.configPath = manifest
			test.options.defaultConfig = manifest
			test.options.repoRoot = repoRoot
			if _, err := prepareInTreeBuildPlan(test.options, nil); err == nil || !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("error = %v, want message %q", err, test.wantMessage)
			}
		})
	}

	t.Run("skip generation with custom manifest", func(t *testing.T) {
		repoRoot := t.TempDir()
		defaultManifest := writeBuildPlanManifest(t, repoRoot, "dist: {}\n")
		customManifest := filepath.Join(repoRoot, "custom.yaml")
		if err := os.WriteFile(customManifest, []byte("dist: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := prepareInTreeBuildPlan(buildPlanOptions{
			configPath:     customManifest,
			defaultConfig:  defaultManifest,
			repoRoot:       repoRoot,
			skipGeneration: true,
		}, nil)
		if err == nil || !strings.Contains(err.Error(), "custom manifests require collector generation") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRunBuildPlanDelegatesWithPlanEnvironmentAndCleansUp(t *testing.T) {
	repoRoot := t.TempDir()
	manifest := writeBuildPlanManifest(t, repoRoot, "alloy_components: [remote.http]\n")
	var delegatedConfig, delegatedOutput string
	fake := &fakeDelegate{inspect: func(invocation delegateInvocation) {
		delegatedConfig = environmentValue(t, invocation.Env, buildPlanConfigEnv)
		delegatedOutput = environmentValue(t, invocation.Env, buildPlanOutputPathEnv)
		if invocation.Name != "make" || !reflect.DeepEqual(invocation.Args, []string{"internal-target"}) {
			t.Fatalf("invocation = %#v", invocation)
		}
		if got := environmentValue(t, invocation.Env, buildPlanBuildTagsEnv); got != "gore2regex alloy_custom_components alloy_component_remote_http" {
			t.Fatalf("plan tags = %q", got)
		}
		if _, err := os.Stat(delegatedConfig); err != nil {
			t.Fatalf("temporary manifest unavailable during delegation: %v", err)
		}
	}}
	var stderr bytes.Buffer
	code := runBuildPlan([]string{
		"--config", manifest,
		"--default-config", manifest,
		"--repo-root", repoRoot,
		"--build-tags", "gore2regex",
		"--", "make", "internal-target",
	}, strings.NewReader(""), io.Discard, &stderr, []string{"A=B"}, fake)
	if code != 0 || fake.calls != 1 {
		t.Fatalf("runBuildPlan()=%d calls=%d stderr=%s", code, fake.calls, stderr.String())
	}
	if _, err := os.Stat(delegatedConfig); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary manifest was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(delegatedOutput)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("temporary workspace was not removed: %v", err)
	}
	if !strings.Contains(stderr.String(), "native Alloy components: remote.http (builder manifest)") {
		t.Fatalf("selection summary missing from stderr: %q", stderr.String())
	}
}

func writeBuildPlanManifest(t *testing.T, repoRoot, contents string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repoRoot, "syntax"), 0o755); err != nil {
		t.Fatalf("create syntax directory: %v", err)
	}
	path := filepath.Join(repoRoot, "builder-config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write builder manifest: %v", err)
	}
	return path
}

func assertMappingString(t *testing.T, mapping *yaml.Node, name, want string) {
	t.Helper()
	node, found := mappingValue(mapping, name)
	if !found || node.Kind != yaml.ScalarNode || node.Value != want {
		t.Fatalf("mapping value %q = %#v, want %q", name, node, want)
	}
}

func assertReplacement(t *testing.T, root *yaml.Node, module, wantTarget string) {
	t.Helper()
	replacements, found := mappingValue(root, "replaces")
	if !found {
		t.Fatal("rewritten manifest has no replaces list")
	}
	for _, replacement := range replacements.Content {
		if replacementModule(replacement.Value) != module {
			continue
		}
		_, target, _ := strings.Cut(replacement.Value, "=>")
		if got := strings.TrimSpace(target); got != wantTarget {
			t.Fatalf("replacement %q target = %q, want %q", module, got, wantTarget)
		}
		return
	}
	t.Fatalf("replacement %q not found", module)
}

func environmentValue(t *testing.T, environ []string, name string) string {
	t.Helper()
	prefix := name + "="
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			return strings.TrimPrefix(entry, prefix)
		}
	}
	t.Fatalf("environment does not contain %s: %#v", name, environ)
	return ""
}
