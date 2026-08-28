package preparehelm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var stableAlloyTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

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
	chartPath               string
	repoRoot                string
	getAlloyRelease         func(ctx context.Context, tag string) (draft, prerelease bool, err error)
	helmChartTagExists      func(ctx context.Context, tag string) (bool, error)
	helmChartsReleaseExists func(ctx context.Context, tag string) (bool, error)
	readChart               func(path string) (chart, error)
	headSHA                 func() (string, error)
}

func (e evaluator) evaluate(ctx context.Context, tag string) (Result, error) {
	if !stableAlloyTag.MatchString(tag) {
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

	sourceTag := "helm-chart/" + ch.Version
	exists, err := e.helmChartTagExists(ctx, sourceTag)
	if err != nil {
		return Result{}, err
	}
	if !exists {
		return Result{}, fmt.Errorf("release-please did not create the expected %s source tag", sourceTag)
	}

	packageName := ch.Name + "-" + ch.Version
	exists, err = e.helmChartsReleaseExists(ctx, packageName)
	if err != nil {
		return Result{}, err
	}
	if exists {
		fmt.Printf("Chart release %s already exists; skipping Helm release.\n", packageName)
		return Result{}, nil
	}

	sha, err := e.headSHA()
	if err != nil {
		return Result{}, err
	}

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
