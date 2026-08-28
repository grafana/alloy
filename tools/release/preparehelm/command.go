package preparehelm

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

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
			"matches, the helm-chart/<version> source tag exists, and grafana/helm-charts does " +
			"not already have alloy-<version>. Writes GITHUB_OUTPUT for the Helm release workflow.",
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
		helmChartTagExists:      alloyClient.HasTag,
		helmChartsReleaseExists: helmClient.HasRelease,
		readChart:               readChart,
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
