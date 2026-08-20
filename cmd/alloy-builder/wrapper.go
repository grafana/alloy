package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/grafana/alloy/internal/component/all/catalog"
	"gopkg.in/yaml.v3"
)

const (
	defaultOCBVersion = "v0.139.0"
	ocbModule         = "go.opentelemetry.io/collector/cmd/builder"
	buildTagsEnv      = "dist.build_tags"
	alloyEngineImport = "github.com/grafana/alloy/extension/alloyengine"
	yamlStringTag     = "!!str"
)

type delegateInvocation struct {
	Name   string
	Args   []string
	Env    []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type delegate interface {
	Run(delegateInvocation) (int, error)
}

type execDelegate struct{}

func (execDelegate) Run(invocation delegateInvocation) (int, error) {
	// #nosec G204 -- the executable is either Go or an explicit user-provided OCB path.
	cmd := exec.Command(invocation.Name, invocation.Args...)
	cmd.Env = invocation.Env
	cmd.Stdin = invocation.Stdin
	cmd.Stdout = invocation.Stdout
	cmd.Stderr = invocation.Stderr
	if err := cmd.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return exitError.ExitCode(), nil
		}
		return 0, err
	}
	return 0, nil
}

type configArgument struct {
	path   string
	index  int
	inline bool
}

type argumentPlan struct {
	delegateArgs []string
	config       *configArgument
	ocbPath      string
	ocbVersion   string
	skipCompile  bool
}

func run(
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
	environ []string,
	runner delegate,
) int {

	plan, err := parseArguments(args)
	if err != nil {
		reportf(stderr, "alloy-builder: %v\n", err)
		return 1
	}

	delegateEnv := environ
	var cleanup func() error
	if plan.config != nil && plan.config.path != "" {
		result, err := prepareManifest(plan.config.path, environ)
		if err != nil {
			reportf(stderr, "alloy-builder: %v\n", err)
			return 1
		}
		if result.changed {
			if plan.skipCompile {
				if err := result.cleanup(); err != nil {
					reportf(stderr, "alloy-builder: remove temporary manifest: %v\n", err)
				}
				reportf(stderr, "alloy-builder: alloy_components cannot be used with --skip-compilation because the injected build tags would not be applied to a later build\n")
				return 1
			}
			plan.replaceConfig(result.path)
			delegateEnv = result.environ
			cleanup = result.cleanup
			reportf(stderr, "alloy-builder: effective build tags: %s\n", result.buildTags)
		}
	}

	if cleanup != nil {
		defer func() {
			if err := cleanup(); err != nil {
				reportf(stderr, "alloy-builder: remove temporary manifest: %v\n", err)
			}
		}()
	}

	invocation := delegateInvocation{
		Env:    delegateEnv,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	if plan.ocbPath != "" {
		invocation.Name = plan.ocbPath
		invocation.Args = plan.delegateArgs
	} else {
		invocation.Name = "go"
		invocation.Args = append([]string{"run", ocbModule + "@" + plan.ocbVersion}, plan.delegateArgs...)
	}

	exitCode, err := runner.Run(invocation)
	if err != nil {
		reportf(stderr, "alloy-builder: start OCB delegate: %v\n", err)
		return 1
	}
	return exitCode
}

func parseArguments(args []string) (argumentPlan, error) {
	plan := argumentPlan{
		delegateArgs: make([]string, 0, len(args)),
		ocbVersion:   defaultOCBVersion,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			plan.delegateArgs = append(plan.delegateArgs, args[i:]...)
			break
		}

		switch {
		case arg == "--ocb":
			if i+1 >= len(args) {
				return argumentPlan{}, errors.New("--ocb requires an executable path")
			}
			i++
			plan.ocbPath = args[i]
			if plan.ocbPath == "" {
				return argumentPlan{}, errors.New("--ocb requires a non-empty executable path")
			}

		case strings.HasPrefix(arg, "--ocb="):
			plan.ocbPath = strings.TrimPrefix(arg, "--ocb=")
			if plan.ocbPath == "" {
				return argumentPlan{}, errors.New("--ocb requires a non-empty executable path")
			}

		case arg == "--ocb-version":
			if i+1 >= len(args) {
				return argumentPlan{}, errors.New("--ocb-version requires a version")
			}
			i++
			plan.ocbVersion = args[i]
			if plan.ocbVersion == "" {
				return argumentPlan{}, errors.New("--ocb-version requires a non-empty version")
			}

		case strings.HasPrefix(arg, "--ocb-version="):
			plan.ocbVersion = strings.TrimPrefix(arg, "--ocb-version=")
			if plan.ocbVersion == "" {
				return argumentPlan{}, errors.New("--ocb-version requires a non-empty version")
			}

		case arg == "--config":
			plan.delegateArgs = append(plan.delegateArgs, arg)
			if i+1 < len(args) {
				i++
				plan.delegateArgs = append(plan.delegateArgs, args[i])
				plan.config = &configArgument{path: args[i], index: len(plan.delegateArgs) - 1}
			}

		case strings.HasPrefix(arg, "--config="):
			plan.delegateArgs = append(plan.delegateArgs, arg)
			plan.config = &configArgument{
				path:   strings.TrimPrefix(arg, "--config="),
				index:  len(plan.delegateArgs) - 1,
				inline: true,
			}

		case arg == "--ldflags" || arg == "--gcflags" || arg == "--output-path":
			plan.delegateArgs = append(plan.delegateArgs, arg)
			if i+1 < len(args) {
				i++
				plan.delegateArgs = append(plan.delegateArgs, args[i])
			}

		default:
			plan.delegateArgs = append(plan.delegateArgs, arg)
		}
	}

	skipCompile, validSkipCompile := booleanFlagValue(plan.delegateArgs, "--skip-compilation")
	plan.skipCompile = validSkipCompile && skipCompile
	return plan, nil
}

func booleanFlagValue(args []string, name string) (bool, bool) {
	value := false
	valid := true
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			break
		}
		if arg == "--config" || arg == "--ldflags" || arg == "--gcflags" || arg == "--output-path" {
			index++
			continue
		}
		switch {
		case arg == name:
			value = true
		case strings.HasPrefix(arg, name+"="):
			parsed, err := strconv.ParseBool(strings.TrimPrefix(arg, name+"="))
			if err != nil {
				valid = false
				continue
			}
			value = parsed
		}
	}
	return value, valid
}

func (p *argumentPlan) replaceConfig(path string) {
	p.config.path = path
	if p.config.inline {
		p.delegateArgs[p.config.index] = "--config=" + path
		return
	}
	p.delegateArgs[p.config.index] = path
}

type manifestResult struct {
	changed   bool
	path      string
	environ   []string
	buildTags string
	cleanup   func() error
}

func prepareManifest(filename string, environ []string) (manifestResult, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return manifestResult{}, fmt.Errorf("read OCB manifest %q: %w", filename, err)
	}

	document, err := decodeYAMLDocument(data)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return manifestResult{environ: environ}, nil
		}
		return manifestResult{}, fmt.Errorf("parse OCB manifest %q: %w", filename, err)
	}
	root := documentMapping(document)
	if root == nil {
		return manifestResult{environ: environ}, nil
	}
	if !mappingHasKey(root, "alloy_components") {
		return manifestResult{environ: environ}, nil
	}

	if err := validateMappingKeys(document, make(map[*yaml.Node]struct{})); err != nil {
		return manifestResult{}, fmt.Errorf("validate OCB manifest %q: %w", filename, err)
	}
	componentsNode, _ := mappingValue(root, "alloy_components")
	components, err := parseAlloyComponents(componentsNode)
	if err != nil {
		return manifestResult{}, fmt.Errorf("validate alloy_components: %w", err)
	}
	componentTags, err := catalog.ResolveExact(components)
	if err != nil {
		return manifestResult{}, fmt.Errorf("validate alloy_components: %w", err)
	}
	if err := requireAlloyEngine(root); err != nil {
		return manifestResult{}, err
	}

	fileBuildTags, err := manifestBuildTags(root)
	if err != nil {
		return manifestResult{}, err
	}
	childEnv, environmentBuildTags, environmentOverrides := removeBuildTagsEnvironment(environ)
	effectiveBuildTags := fileBuildTags
	if environmentOverrides {
		effectiveBuildTags = environmentBuildTags
	}
	unrelatedTags, err := validateUnrelatedBuildTags(effectiveBuildTags)
	if err != nil {
		return manifestResult{}, err
	}

	allTags := make([]string, 0, len(unrelatedTags)+len(componentTags)+1)
	allTags = append(allTags, unrelatedTags...)
	allTags = append(allTags, catalog.CustomBuildTag)
	allTags = append(allTags, componentTags...)
	setManifestBuildTags(root, strings.Join(allTags, ","))
	removeMappingKey(root, "alloy_components")

	rewritten, err := encodeYAMLDocument(document)
	if err != nil {
		return manifestResult{}, fmt.Errorf("encode OCB manifest %q: %w", filename, err)
	}
	temporaryPath, cleanup, err := writeTemporaryManifest(rewritten)
	if err != nil {
		return manifestResult{}, err
	}
	return manifestResult{
		changed:   true,
		path:      temporaryPath,
		environ:   childEnv,
		buildTags: strings.Join(allTags, ","),
		cleanup:   cleanup,
	}, nil
}

func decodeYAMLDocument(data []byte) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, errors.New("multiple YAML documents are not supported")
		}
		return nil, err
	}
	return &document, nil
}

func encodeYAMLDocument(document *yaml.Node) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(document); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func documentMapping(document *yaml.Node) *yaml.Node {
	if document == nil || document.Kind != yaml.DocumentNode || len(document.Content) != 1 {
		return nil
	}
	if document.Content[0].Kind != yaml.MappingNode {
		return nil
	}
	return document.Content[0]
}

func validateMappingKeys(node *yaml.Node, visited map[*yaml.Node]struct{}) error {
	if node == nil {
		return nil
	}
	if _, exists := visited[node]; exists {
		return nil
	}
	visited[node] = struct{}{}

	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAML aliases are not supported when alloy_components is set (alias at line %d)", node.Line)
	}
	if node.Kind == yaml.MappingNode {
		if len(node.Content)%2 != 0 {
			return fmt.Errorf("mapping at line %d has an unmatched key", node.Line)
		}
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != yamlStringTag {
				return fmt.Errorf("mapping key at line %d must be a string", key.Line)
			}
			if _, exists := seen[key.Value]; exists {
				return fmt.Errorf("duplicate mapping key %q at line %d", key.Value, key.Line)
			}
			seen[key.Value] = struct{}{}
			if err := validateMappingKeys(node.Content[i+1], visited); err != nil {
				return err
			}
		}
		return nil
	}
	for _, child := range node.Content {
		if err := validateMappingKeys(child, visited); err != nil {
			return err
		}
	}
	return nil
}

func mappingHasKey(mapping *yaml.Node, name string) bool {
	_, found := mappingValue(mapping, name)
	return found
}

func mappingValue(mapping *yaml.Node, name string) (*yaml.Node, bool) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil, false
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind == yaml.ScalarNode && key.Tag == yamlStringTag && key.Value == name {
			return mapping.Content[i+1], true
		}
	}
	return nil, false
}

func removeMappingKey(mapping *yaml.Node, name string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Kind == yaml.ScalarNode && key.Tag == yamlStringTag && key.Value == name {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

func parseAlloyComponents(node *yaml.Node) ([]string, error) {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil, errors.New("must be a YAML list (use [] to select no native components)")
	}
	components := make([]string, 0, len(node.Content))
	for index, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != yamlStringTag || item.Value == "" {
			return nil, fmt.Errorf("item %d must be a non-empty string", index)
		}
		components = append(components, item.Value)
	}
	return components, nil
}

func requireAlloyEngine(root *yaml.Node) error {
	extensions, found := mappingValue(root, "extensions")
	if !found || extensions.Kind != yaml.SequenceNode {
		return fmt.Errorf("alloy_components requires an extensions list containing import %q", alloyEngineImport)
	}
	for _, extension := range extensions.Content {
		if extension.Kind != yaml.MappingNode {
			continue
		}
		importNode, found := mappingValue(extension, "import")
		if found && importNode.Kind == yaml.ScalarNode && importNode.Tag == yamlStringTag && importNode.Value == alloyEngineImport {
			return nil
		}
	}
	return fmt.Errorf("alloy_components requires the alloyengine extension import %q", alloyEngineImport)
}

func manifestBuildTags(root *yaml.Node) (string, error) {
	distribution, found := mappingValue(root, "dist")
	if !found {
		return "", nil
	}
	if distribution.Kind != yaml.MappingNode {
		return "", errors.New("dist must be a mapping")
	}
	buildTags, found := mappingValue(distribution, "build_tags")
	if !found {
		return "", nil
	}
	if buildTags.Kind != yaml.ScalarNode || buildTags.Tag != yamlStringTag {
		return "", errors.New("dist.build_tags must be a string")
	}
	return buildTags.Value, nil
}

func setManifestBuildTags(root *yaml.Node, value string) {
	distribution, found := mappingValue(root, "dist")
	if !found {
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "dist"},
			&yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"},
		)
		distribution = root.Content[len(root.Content)-1]
	}
	buildTags, found := mappingValue(distribution, "build_tags")
	if !found {
		distribution.Content = append(distribution.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag, Value: "build_tags"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: yamlStringTag},
		)
		buildTags = distribution.Content[len(distribution.Content)-1]
	}
	buildTags.Kind = yaml.ScalarNode
	buildTags.Tag = yamlStringTag
	buildTags.Style = 0
	buildTags.Value = value
}

func validateUnrelatedBuildTags(value string) ([]string, error) {
	return validateBuildTagList(splitBuildTags(value), "dist.build_tags")
}

func validateBuildTagList(tags []string, source string) ([]string, error) {
	unique := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if tag == catalog.CustomBuildTag || strings.HasPrefix(tag, "alloy_component_") {
			return nil, fmt.Errorf("%s contains reserved Alloy build tag %q; configure native components with alloy_components instead", source, tag)
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		unique = append(unique, tag)
	}
	return unique, nil
}

func splitBuildTags(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

func removeBuildTagsEnvironment(environ []string) ([]string, string, bool) {
	prefix := buildTagsEnv + "="
	filtered := make([]string, 0, len(environ))
	value := ""
	found := false
	for _, entry := range environ {
		if strings.HasPrefix(entry, prefix) {
			value = strings.TrimPrefix(entry, prefix)
			found = true
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, value, found
}

func writeTemporaryManifest(data []byte) (string, func() error, error) {
	file, err := os.CreateTemp("", "alloy-builder-*.yaml")
	if err != nil {
		return "", nil, fmt.Errorf("create private temporary OCB manifest: %w", err)
	}
	temporaryPath := file.Name()
	cleanup := func() error { return os.Remove(temporaryPath) }
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = cleanup()
		return "", nil, fmt.Errorf("write temporary OCB manifest %q: %w", temporaryPath, err)
	}
	if err := file.Close(); err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("close temporary OCB manifest %q: %w", temporaryPath, err)
	}
	return temporaryPath, cleanup, nil
}

func reportf(writer io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(writer, format, args...)
}
