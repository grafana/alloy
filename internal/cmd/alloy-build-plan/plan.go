package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/grafana/alloy/internal/component/all/catalog"
	"gopkg.in/yaml.v3"
)

const (
	inTreeModule = "github.com/grafana/alloy/otel_engine"

	buildPlanConfigEnv         = "ALLOY_BUILD_PLAN_CONFIG"
	buildPlanOutputPathEnv     = "ALLOY_BUILD_PLAN_OUTPUT_PATH"
	buildPlanBuildTagsEnv      = "ALLOY_BUILD_PLAN_TAGS"
	buildPlanNeedsBeylaEnv     = "ALLOY_BUILD_PLAN_NEEDS_BEYLA"
	buildPlanSkipGenerationEnv = "ALLOY_BUILD_PLAN_SKIP_GENERATION"

	nativeModeFull   = "full"
	nativeModeCustom = "custom"
)

type buildPlanOptions struct {
	configPath     string
	defaultConfig  string
	repoRoot       string
	buildTags      string
	skipGeneration bool
}

type nativeSelection struct {
	mode       string
	components []string
	buildTags  []string
	source     string
}

type preparedBuildPlan struct {
	configPath     string
	outputPath     string
	buildTags      []string
	selection      nativeSelection
	needsBeyla     bool
	skipGeneration bool
	environ        []string
	cleanup        func() error
}

func runBuildPlan(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	environ []string,
	runner delegate,
) int {

	options, command, err := parseBuildPlanArguments(args)
	if err != nil {
		reportf(stderr, "alloy-build-plan: %v\n", err)
		return 1
	}

	plan, err := prepareInTreeBuildPlan(options, environ)
	if err != nil {
		reportf(stderr, "alloy-build-plan: %v\n", err)
		return 1
	}
	if plan.cleanup != nil {
		defer func() {
			if err := plan.cleanup(); err != nil {
				reportf(stderr, "alloy-build-plan: remove temporary build workspace: %v\n", err)
			}
		}()
	}

	componentSummary := "all"
	if plan.selection.mode == nativeModeCustom {
		componentSummary = "none"
		if len(plan.selection.components) > 0 {
			componentSummary = strings.Join(plan.selection.components, ",")
		}
	}
	reportf(stderr, "alloy-build-plan: native Alloy components: %s (%s)\n", componentSummary, plan.selection.source)
	reportf(stderr, "alloy-build-plan: effective build tags: %s\n", strings.Join(plan.buildTags, ","))

	childEnv := setEnvironmentValues(plan.environ, map[string]string{
		buildPlanConfigEnv:         plan.configPath,
		buildPlanOutputPathEnv:     plan.outputPath,
		buildPlanBuildTagsEnv:      strings.Join(plan.buildTags, " "),
		buildPlanNeedsBeylaEnv:     strconv.FormatBool(plan.needsBeyla),
		buildPlanSkipGenerationEnv: strconv.FormatBool(plan.skipGeneration),
	})

	exitCode, err := runner.Run(delegateInvocation{
		Name:   command[0],
		Args:   command[1:],
		Env:    childEnv,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		reportf(stderr, "alloy-build-plan: start build command: %v\n", err)
		return 1
	}
	return exitCode
}

func parseBuildPlanArguments(args []string) (buildPlanOptions, []string, error) {
	separator := -1
	for index, arg := range args {
		if arg == "--" {
			separator = index
			break
		}
	}
	if separator < 0 {
		return buildPlanOptions{}, nil, errors.New("a child build command must follow --")
	}
	if separator == len(args)-1 {
		return buildPlanOptions{}, nil, errors.New("a child build command must follow --")
	}

	options := buildPlanOptions{}
	flags := flag.NewFlagSet("alloy-build-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&options.configPath, "config", "collector/builder-config.yaml", "builder manifest")
	flags.StringVar(&options.defaultConfig, "default-config", "collector/builder-config.yaml", "implicit default builder manifest")
	flags.StringVar(&options.repoRoot, "repo-root", ".", "Alloy repository root")
	flags.StringVar(&options.buildTags, "build-tags", "", "additional Go build tags")
	flags.BoolVar(&options.skipGeneration, "skip-generation", false, "build checked-in generated collector sources")

	if err := flags.Parse(args[:separator]); err != nil {
		return buildPlanOptions{}, nil, err
	}
	if len(flags.Args()) != 0 {
		return buildPlanOptions{}, nil, fmt.Errorf("unexpected plan argument %q", flags.Args()[0])
	}
	return options, args[separator+1:], nil
}

func prepareInTreeBuildPlan(options buildPlanOptions, environ []string) (preparedBuildPlan, error) {
	repoRoot, err := filepath.Abs(options.repoRoot)
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("resolve repository root %q: %w", options.repoRoot, err)
	}
	configPath, err := filepath.Abs(options.configPath)
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("resolve builder manifest %q: %w", options.configPath, err)
	}
	defaultConfig, err := filepath.Abs(options.defaultConfig)
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("resolve default builder manifest %q: %w", options.defaultConfig, err)
	}
	if options.skipGeneration && !samePath(configPath, defaultConfig) {
		return preparedBuildPlan{}, fmt.Errorf("--skip-generation can only use the implicit default manifest %q; custom manifests require collector generation", defaultConfig)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("read builder manifest %q: %w", configPath, err)
	}
	document, err := decodeYAMLDocument(data)
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("parse builder manifest %q: %w", configPath, err)
	}
	root := documentMapping(document)
	if root == nil {
		return preparedBuildPlan{}, fmt.Errorf("builder manifest %q must contain one YAML mapping", configPath)
	}
	if err := validateMappingKeys(document, make(map[*yaml.Node]struct{})); err != nil {
		return preparedBuildPlan{}, fmt.Errorf("validate builder manifest %q: %w", configPath, err)
	}
	if !samePath(configPath, defaultConfig) {
		defaultRoot, err := readBuildManifestRoot(defaultConfig)
		if err != nil {
			return preparedBuildPlan{}, err
		}
		if err := mergeInTreeBuildPolicy(root, defaultRoot); err != nil {
			return preparedBuildPlan{}, fmt.Errorf("merge in-tree build policy from %q: %w", defaultConfig, err)
		}
	}

	fileBuildTags, err := manifestBuildTags(root)
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("validate builder manifest %q: %w", configPath, err)
	}
	childEnv, environmentBuildTags, environmentOverrides := removeBuildTagsEnvironment(environ)
	if environmentOverrides {
		fileBuildTags = environmentBuildTags
	}
	baseTags := append(splitBuildTags(fileBuildTags), splitBuildTags(options.buildTags)...)
	selection, effectiveTags, err := resolveNativeSelection(root, baseTags)
	if err != nil {
		return preparedBuildPlan{}, err
	}

	setManifestBuildTags(root, strings.Join(effectiveTags, ","))
	removeMappingKey(root, "alloy_components")
	replacementBase, err := replacementBasePath(root, repoRoot)
	if err != nil {
		return preparedBuildPlan{}, err
	}
	if err := setInTreeDistribution(root); err != nil {
		return preparedBuildPlan{}, err
	}
	if err := setInTreeReplacements(root, repoRoot, replacementBase); err != nil {
		return preparedBuildPlan{}, err
	}

	plan := preparedBuildPlan{
		configPath:     configPath,
		outputPath:     filepath.Join(repoRoot, "collector"),
		buildTags:      effectiveTags,
		selection:      selection,
		needsBeyla:     selection.mode == nativeModeFull || containsString(selection.components, "beyla.ebpf"),
		skipGeneration: options.skipGeneration,
		environ:        childEnv,
	}
	if options.skipGeneration {
		return plan, nil
	}

	buildRoot := filepath.Join(repoRoot, "build")
	if err := os.MkdirAll(buildRoot, 0o755); err != nil {
		return preparedBuildPlan{}, fmt.Errorf("create build directory %q: %w", buildRoot, err)
	}
	workspace, err := os.MkdirTemp(buildRoot, ".alloy-build-plan-")
	if err != nil {
		return preparedBuildPlan{}, fmt.Errorf("create temporary build workspace in %q: %w", buildRoot, err)
	}
	cleanup := func() error { return os.RemoveAll(workspace) }
	plan.cleanup = cleanup
	plan.outputPath = filepath.Join(workspace, "collector")
	if err := setDistributionString(root, "output_path", plan.outputPath); err != nil {
		_ = cleanup()
		return preparedBuildPlan{}, err
	}

	rewritten, err := encodeYAMLDocument(document)
	if err != nil {
		_ = cleanup()
		return preparedBuildPlan{}, fmt.Errorf("encode builder manifest %q: %w", configPath, err)
	}
	plan.configPath = filepath.Join(workspace, "builder-config.yaml")
	if err := os.WriteFile(plan.configPath, rewritten, 0o600); err != nil {
		_ = cleanup()
		return preparedBuildPlan{}, fmt.Errorf("write temporary builder manifest %q: %w", plan.configPath, err)
	}
	return plan, nil
}

func resolveNativeSelection(root *yaml.Node, baseTags []string) (nativeSelection, []string, error) {
	componentsNode, manifestSelection := mappingValue(root, "alloy_components")
	unrelatedTags, err := validateBuildTagList(baseTags, "effective build tags")
	if err != nil {
		return nativeSelection{}, nil, err
	}
	selection := nativeSelection{mode: nativeModeFull, source: "alloy_components omitted"}

	if manifestSelection {
		components, err := parseAlloyComponents(componentsNode)
		if err != nil {
			return nativeSelection{}, nil, fmt.Errorf("validate alloy_components: %w", err)
		}
		componentTags, err := catalog.ResolveExact(components)
		if err != nil {
			return nativeSelection{}, nil, fmt.Errorf("validate native component selection: %w", err)
		}
		selection.mode = nativeModeCustom
		selection.source = "builder manifest"
		selection.components = append([]string(nil), components...)
		sort.Strings(selection.components)
		selection.buildTags = append([]string{catalog.CustomBuildTag}, componentTags...)
	}

	effectiveTags := append([]string(nil), unrelatedTags...)
	effectiveTags = append(effectiveTags, selection.buildTags...)
	return selection, effectiveTags, nil
}

func setInTreeDistribution(root *yaml.Node) error {
	// The generated module must live beneath the Alloy import path so it can use
	// internal packages. The output path is set after the private workspace is
	// allocated.
	return setDistributionString(root, "module", inTreeModule)
}

func setDistributionString(root *yaml.Node, name, value string) error {
	distribution, found := mappingValue(root, "dist")
	if !found {
		distribution = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "dist"},
			distribution,
		)
	}
	if distribution.Kind != yaml.MappingNode {
		return errors.New("dist must be a mapping")
	}
	setMappingString(distribution, name, value)
	return nil
}

func setMappingString(mapping *yaml.Node, name, value string) {
	node, found := mappingValue(mapping, name)
	if !found {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: name},
			&yaml.Node{},
		)
		node = mapping.Content[len(mapping.Content)-1]
	}
	node.Kind = yaml.ScalarNode
	node.Tag = yamlStringTag
	node.Style = 0
	node.Value = value
}

func replacementBasePath(root *yaml.Node, repoRoot string) (string, error) {
	distribution, found := mappingValue(root, "dist")
	if !found {
		return repoRoot, nil
	}
	if distribution.Kind != yaml.MappingNode {
		return "", errors.New("dist must be a mapping")
	}
	outputPath, found := mappingValue(distribution, "output_path")
	if !found {
		return repoRoot, nil
	}
	if outputPath.Kind != yaml.ScalarNode || outputPath.Tag != yamlStringTag {
		return "", errors.New("dist.output_path must be a string")
	}
	if outputPath.Value == "" {
		return repoRoot, nil
	}
	if filepath.IsAbs(outputPath.Value) {
		return filepath.Clean(outputPath.Value), nil
	}
	return filepath.Clean(filepath.Join(repoRoot, outputPath.Value)), nil
}

func setInTreeReplacements(root *yaml.Node, repoRoot, replacementBase string) error {
	replacements, found := mappingValue(root, "replaces")
	if !found {
		replacements = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "replaces"},
			replacements,
		)
	}
	if replacements.Kind != yaml.SequenceNode {
		return errors.New("replaces must be a YAML list")
	}

	modules := map[string]string{
		"github.com/grafana/alloy":        repoRoot,
		"github.com/grafana/alloy/syntax": filepath.Join(repoRoot, "syntax"),
	}
	preserved := replacements.Content[:0]
	for _, replacement := range replacements.Content {
		if replacement.Kind != yaml.ScalarNode || replacement.Tag != yamlStringTag {
			return errors.New("every replaces item must be a string")
		}
		module := replacementModule(replacement.Value)
		if _, managed := modules[module]; managed {
			continue
		}
		rebased, err := rebaseLocalReplacement(replacement.Value, replacementBase)
		if err != nil {
			return err
		}
		replacement.Value = rebased
		preserved = append(preserved, replacement)
	}
	replacements.Content = preserved

	for _, module := range []string{"github.com/grafana/alloy", "github.com/grafana/alloy/syntax"} {
		target := formatReplacementPath(modules[module])
		replacements.Content = append(replacements.Content, &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   yamlStringTag,
			Value: module + " => " + target,
		})
	}
	return nil
}

func rebaseLocalReplacement(value, base string) (string, error) {
	left, right, found := strings.Cut(value, "=>")
	if !found {
		return value, nil
	}
	right = strings.TrimSpace(right)
	path := right
	if strings.HasPrefix(right, `"`) {
		unquoted, err := strconv.Unquote(right)
		if err != nil {
			return value, nil
		}
		path = unquoted
	} else if len(strings.Fields(right)) != 1 {
		// A module replacement has both a module path and a version.
		return value, nil
	}
	if !isLocalReplacementPath(path) || filepath.IsAbs(path) {
		return value, nil
	}
	absolute, err := filepath.Abs(filepath.Join(base, path))
	if err != nil {
		return "", fmt.Errorf("resolve local replacement %q: %w", path, err)
	}
	return strings.TrimSpace(left) + " => " + formatReplacementPath(absolute), nil
}

func isLocalReplacementPath(path string) bool {
	return path == "." || path == ".." ||
		strings.HasPrefix(path, "./") || strings.HasPrefix(path, "../") ||
		strings.HasPrefix(path, `.\`) || strings.HasPrefix(path, `..\`) ||
		filepath.IsAbs(path)
}

func formatReplacementPath(path string) string {
	path = filepath.ToSlash(path)
	if strings.ContainsAny(path, " \t") {
		return strconv.Quote(path)
	}
	return path
}

func readBuildManifestRoot(filename string) (*yaml.Node, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read default builder manifest %q: %w", filename, err)
	}
	document, err := decodeYAMLDocument(data)
	if err != nil {
		return nil, fmt.Errorf("parse default builder manifest %q: %w", filename, err)
	}
	root := documentMapping(document)
	if root == nil {
		return nil, fmt.Errorf("default builder manifest %q must contain one YAML mapping", filename)
	}
	return root, nil
}

func mergeInTreeBuildPolicy(root, defaults *yaml.Node) error {
	if err := mergeManifestSequence(root, defaults, "replaces", replacementModule); err != nil {
		return err
	}
	return mergeManifestSequence(root, defaults, "excludes", func(value string) string {
		return strings.TrimSpace(value)
	})
}

func mergeManifestSequence(root, defaults *yaml.Node, name string, identity func(string) string) error {
	defaultSequence, found := mappingValue(defaults, name)
	if !found {
		return nil
	}
	if defaultSequence.Kind != yaml.SequenceNode {
		return fmt.Errorf("default %s must be a YAML list", name)
	}
	sequence, found := mappingValue(root, name)
	if !found {
		sequence = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: name},
			sequence,
		)
	}
	if sequence.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s must be a YAML list", name)
	}

	seen := make(map[string]struct{}, len(sequence.Content))
	for _, item := range sequence.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != yamlStringTag {
			return fmt.Errorf("every %s item must be a string", name)
		}
		seen[identity(item.Value)] = struct{}{}
	}
	for _, item := range defaultSequence.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != yamlStringTag {
			return fmt.Errorf("every default %s item must be a string", name)
		}
		if _, exists := seen[identity(item.Value)]; exists {
			continue
		}
		sequence.Content = append(sequence.Content, cloneYAMLNode(item))
		seen[identity(item.Value)] = struct{}{}
	}
	return nil
}

func cloneYAMLNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	clone := *node
	clone.Content = make([]*yaml.Node, len(node.Content))
	for index, child := range node.Content {
		clone.Content[index] = cloneYAMLNode(child)
	}
	return &clone
}

func replacementModule(value string) string {
	left, _, found := strings.Cut(value, "=>")
	if !found {
		return ""
	}
	fields := strings.Fields(left)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func setEnvironmentValues(environ []string, values map[string]string) []string {
	result := make([]string, 0, len(environ)+len(values))
	for _, entry := range environ {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, replaced := values[name]; replaced {
				continue
			}
		}
		result = append(result, entry)
	}
	for _, name := range []string{
		buildPlanConfigEnv,
		buildPlanOutputPathEnv,
		buildPlanBuildTagsEnv,
		buildPlanNeedsBeylaEnv,
		buildPlanSkipGenerationEnv,
	} {
		result = append(result, name+"="+values[name])
	}
	return result
}

func samePath(left, right string) bool {
	leftEvaluated, leftErr := filepath.EvalSymlinks(left)
	rightEvaluated, rightErr := filepath.EvalSymlinks(right)
	if leftErr == nil && rightErr == nil {
		return leftEvaluated == rightEvaluated
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
