package preparehelm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/tools/release/internal/gha"
	gh "github.com/grafana/alloy/tools/release/internal/github"
)

const (
	defaultChartPath      = "operations/helm/charts/alloy"
	defaultHelmChartsRepo = "grafana/helm-charts"
)

type flags struct {
	tag            string
	chartPath      string
	helmChartsRepo string
	repoRoot       string
}

func Command() *cobra.Command {
	var flags flags

	cmd := &cobra.Command{
		Use:   "prepare-helm",
		Short: "Decide whether to publish the Alloy Helm chart and emit GitHub Actions outputs",
		Long: "Checks that the Alloy tag is a published stable release, Chart.yaml appVersion " +
			"matches, helm-chart/<version> points to the Alloy release commit, and the chart is " +
			"not already indexed in grafana/helm-charts. Writes GITHUB_OUTPUT for the Helm release workflow.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.Flags().StringVar(&flags.tag, "tag", "", "Alloy release tag (e.g. v1.19.2)")
	cmd.Flags().StringVar(&flags.chartPath, "chart-path", defaultChartPath, "Path to the Helm chart, relative to --repo-root")
	cmd.Flags().StringVar(&flags.helmChartsRepo, "helm-charts-repo", defaultHelmChartsRepo, "owner/repo that hosts published charts")
	cmd.Flags().StringVar(&flags.repoRoot, "repo-root", ".", "Alloy checkout containing Chart.yaml (cwd-relative unless absolute)")
	_ = cmd.MarkFlagRequired("tag")

	return cmd
}

func run(ctx context.Context, flags flags) error {
	alloyClient, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	helmOwner, helmRepo, err := splitRepo(flags.helmChartsRepo)
	if err != nil {
		return err
	}
	helmClient := gh.NewClient(ctx, gh.ClientConfig{
		Token: os.Getenv("GITHUB_TOKEN"),
		Owner: helmOwner,
		Repo:  helmRepo,
	})

	e := evaluator{
		chartPath: flags.chartPath,
		repoRoot:  flags.repoRoot,
		getAlloyRelease: func(ctx context.Context, tag string) (bool, bool, error) {
			release, err := alloyClient.GetReleaseByTag(ctx, tag)
			if err != nil {
				return false, false, err
			}
			return release.GetDraft(), release.GetPrerelease(), nil
		},
		helmChartTagSHA:  alloyClient.GetRefSHA,
		helmChartIndexed: helmChartIndexed(helmClient),
		readChart:        readChart,
		headSHA: func() (string, error) {
			return headSHA(flags.repoRoot)
		},
	}

	result, err := e.evaluate(ctx, flags.tag)
	if err != nil {
		return err
	}

	return writeOutputs(result)
}

type helmIndex struct {
	Entries map[string][]helmIndexEntry `yaml:"entries"`
}

type helmIndexEntry struct {
	Version string `yaml:"version"`
}

func helmChartIndexed(client *gh.Client) func(context.Context, string, string) (bool, error) {
	return func(ctx context.Context, name, version string) (bool, error) {
		content, _, _, err := client.API().Repositories.GetContents(ctx, client.Owner(), client.Repo(), "index.yaml", &github.RepositoryContentGetOptions{Ref: "gh-pages"})
		if err != nil {
			return false, fmt.Errorf("getting Helm chart index: %w", err)
		}

		data, err := content.GetContent()
		if err != nil {
			return false, fmt.Errorf("reading Helm chart index: %w", err)
		}
		return chartVersionIndexed([]byte(data), name, version)
	}
}

func chartVersionIndexed(data []byte, name, version string) (bool, error) {
	var index helmIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return false, fmt.Errorf("parsing Helm chart index: %w", err)
	}
	for _, entry := range index.Entries[name] {
		if entry.Version == version {
			return true, nil
		}
	}
	return false, nil
}

func splitRepo(slug string) (owner, repo string, err error) {
	owner, repo, ok := strings.Cut(slug, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("invalid helm-charts repo %q (expected owner/repo)", slug)
	}
	return owner, repo, nil
}

func writeOutputs(result Result) error {
	if err := gha.SetOutput("changed", strconv.FormatBool(result.Changed)); err != nil {
		return err
	}
	if !result.Changed {
		return nil
	}

	for name, value := range map[string]string{
		"chartpath":   result.ChartPath,
		"desc":        result.Description,
		"tagname":     result.TagName,
		"packagename": result.PackageName,
		"source_sha":  result.SourceSHA,
	} {
		if err := gha.SetOutput(name, value); err != nil {
			return err
		}
	}
	return nil
}
