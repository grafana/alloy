package preparehelm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvaluateSkipsNonStableTags(t *testing.T) {
	t.Parallel()

	for _, tag := range []string{"v1.19.2-rc.0", "v1.19", "not-a-tag", "v1.19.2+build"} {
		t.Run(tag, func(t *testing.T) {
			t.Parallel()

			result, err := evaluator{}.evaluate(context.Background(), tag)
			require.NoError(t, err)
			require.False(t, result.Changed)
		})
	}
}

func TestEvaluateSkipsDraftOrPrerelease(t *testing.T) {
	t.Parallel()

	e := evaluator{
		getAlloyRelease: func(context.Context, string) (bool, bool, error) {
			return true, false, nil
		},
	}

	result, err := e.evaluate(context.Background(), "v1.19.2")
	require.NoError(t, err)
	require.False(t, result.Changed)
}

func TestEvaluateErrorsWhenAppVersionDoesNotMatch(t *testing.T) {
	t.Parallel()

	e := readyEvaluator(t)
	e.readChart = func(string) (chart, error) {
		return chart{Name: "alloy", Description: "Grafana Alloy", Version: "1.12.1", AppVersion: "v1.19.1"}, nil
	}

	_, err := e.evaluate(context.Background(), "v1.19.2")
	require.ErrorContains(t, err, "appVersion v1.19.1 does not match Alloy release v1.19.2")
}

func TestEvaluateErrorsWhenHelmChartTagIsMissing(t *testing.T) {
	t.Parallel()

	e := readyEvaluator(t)
	e.helmChartTagExists = func(context.Context, string) (bool, error) {
		return false, nil
	}

	_, err := e.evaluate(context.Background(), "v1.19.2")
	require.ErrorContains(t, err, "helm-chart/1.12.1")
}

func TestEvaluateSkipsWhenHelmChartsReleaseExists(t *testing.T) {
	t.Parallel()

	e := readyEvaluator(t)
	e.helmChartsReleaseExists = func(context.Context, string) (bool, error) {
		return true, nil
	}

	result, err := e.evaluate(context.Background(), "v1.19.2")
	require.NoError(t, err)
	require.False(t, result.Changed)
}

func TestEvaluatePreparesANewChartRelease(t *testing.T) {
	t.Parallel()

	e := readyEvaluator(t)
	result, err := e.evaluate(context.Background(), "v1.19.2")
	require.NoError(t, err)
	require.Equal(t, result, Result{
		Changed:     true,
		ChartPath:   defaultChartPath,
		Description: "Grafana Alloy",
		TagName:     "helm-chart/1.12.1",
		PackageName: "alloy-1.12.1",
		SourceSHA:   "abc123",
	})
}

func TestReadChartParsesAppVersionComment(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "Chart.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
name: alloy
description: Grafana Alloy
version: 1.12.1
# x-release-please-start-version
appVersion: "v1.19.2"
# x-release-please-end
`), 0o600))

	got, err := readChart(path)
	require.NoError(t, err)
	require.Equal(t, chart{
		Name:        "alloy",
		Description: "Grafana Alloy",
		Version:     "1.12.1",
		AppVersion:  "v1.19.2",
	}, got)
}

func readyEvaluator(t *testing.T) evaluator {
	t.Helper()
	return evaluator{
		chartPath: defaultChartPath,
		getAlloyRelease: func(context.Context, string) (bool, bool, error) {
			return false, false, nil
		},
		helmChartTagExists: func(_ context.Context, tag string) (bool, error) {
			require.Equal(t, "helm-chart/1.12.1", tag)
			return true, nil
		},
		helmChartsReleaseExists: func(_ context.Context, tag string) (bool, error) {
			require.Equal(t, "alloy-1.12.1", tag)
			return false, nil
		},
		readChart: func(string) (chart, error) {
			return chart{Name: "alloy", Description: "Grafana Alloy", Version: "1.12.1", AppVersion: "v1.19.2"}, nil
		},
		headSHA: func() (string, error) {
			return "abc123", nil
		},
	}
}

func TestEvaluatePropagatesLookupErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	e := evaluator{
		getAlloyRelease: func(context.Context, string) (bool, bool, error) {
			return false, false, boom
		},
	}

	_, err := e.evaluate(context.Background(), "v1.19.2")
	require.ErrorIs(t, err, boom)
}
