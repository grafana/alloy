package preparehelm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

func isStableAlloyTag(tag string) bool {
	return semver.IsValid(tag) && strings.Count(tag, ".") == 2 && semver.Prerelease(tag) == "" && semver.Build(tag) == ""
}

type chart struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"appVersion"`
}

// Result is the GitHub Actions output for a Helm publish decision.
type Result struct {
	Changed     bool
	ChartPath   string
	Description string
	TagName     string
	PackageName string
	SourceSHA   string
}

type evaluator struct {
	chartPath        string
	repoRoot         string
	getAlloyRelease  func(ctx context.Context, tag string) (draft, prerelease bool, err error)
	helmChartTagSHA  func(ctx context.Context, tag string) (string, error)
	helmChartIndexed func(ctx context.Context, name, version string) (bool, error)
	readChart        func(path string) (chart, error)
	headSHA          func() (string, error)
}

func (e evaluator) evaluate(ctx context.Context, tag string) (Result, error) {
	if !isStableAlloyTag(tag) {
		fmt.Printf("Release tag %s is not a stable Alloy version; skipping Helm release.\n", tag)
		return Result{}, nil
	}

	draft, prerelease, err := e.getAlloyRelease(ctx, tag)
	if err != nil {
		return Result{}, err
	}
	if draft || prerelease {
		fmt.Printf("Release %s is not a published stable release; skipping Helm release.\n", tag)
		return Result{}, nil
	}

	chartPath := e.chartPath
	if chartPath == "" {
		chartPath = defaultChartPath
	}

	ch, err := e.readChart(filepath.Join(e.repoRoot, chartPath, "Chart.yaml"))
	if err != nil {
		return Result{}, err
	}
	if ch.AppVersion != tag {
		return Result{}, fmt.Errorf("chart appVersion %s does not match Alloy release %s", ch.AppVersion, tag)
	}

	sha, err := e.headSHA()
	if err != nil {
		return Result{}, err
	}

	sourceTag := "helm-chart/" + ch.Version
	sourceSHA, err := e.helmChartTagSHA(ctx, sourceTag)
	if err != nil {
		return Result{}, err
	}
	if sourceSHA != sha {
		return Result{}, fmt.Errorf("source tag %s points to %s, not Alloy release commit %s", sourceTag, sourceSHA, sha)
	}

	indexed, err := e.helmChartIndexed(ctx, ch.Name, ch.Version)
	if err != nil {
		return Result{}, err
	}
	if indexed {
		fmt.Printf("Chart %s %s is already indexed; skipping Helm release.\n", ch.Name, ch.Version)
		return Result{}, nil
	}

	packageName := ch.Name + "-" + ch.Version

	fmt.Printf("Releasing %s %s for %s.\n", chartPath, ch.Version, tag)
	return Result{
		Changed:     true,
		ChartPath:   chartPath,
		Description: ch.Description,
		TagName:     sourceTag,
		PackageName: packageName,
		SourceSHA:   sha,
	}, nil
}

func readChart(path string) (chart, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return chart{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var ch chart
	if err := yaml.Unmarshal(data, &ch); err != nil {
		return chart{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if ch.Name == "" || ch.Version == "" || ch.AppVersion == "" {
		return chart{}, fmt.Errorf("%s is missing name, version, or appVersion", path)
	}
	return ch, nil
}

func headSHA(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolving HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
