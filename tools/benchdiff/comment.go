package benchdiff

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/internal/prcomment"
)

// Label triggers the Benchmark PR workflow.
const Label = "run-benchmarks"

const maxCommentLen = 63_000

func commentCommand() *cobra.Command {
	var report string

	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Post the benchmark status on the pull request of the current GitHub Actions run",
		Long: `Without --report, posts that the benchmarks are running. With --report,
replaces that comment with the report (or a failure message if the report is
missing or empty) and removes the run-benchmarks label.

Reads GITHUB_TOKEN and the run context from the GitHub Actions environment.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return comment(cmd.Context(), report)
		},
	}
	cmd.Flags().StringVar(&report, "report", "", "Report written by benchdiff run")
	return cmd
}

func comment(ctx context.Context, reportFile string) error {
	var event struct {
		PullRequest struct {
			Number int
			Head   struct{ SHA string }
		} `json:"pull_request"`
	}
	data, err := os.ReadFile(os.Getenv("GITHUB_EVENT_PATH"))
	if err != nil {
		return fmt.Errorf("reading the GitHub event: %w", err)
	}
	if err := json.Unmarshal(data, &event); err != nil {
		return err
	}
	pr := event.PullRequest.Number
	owner, repo, ok := strings.Cut(os.Getenv("GITHUB_REPOSITORY"), "/")
	if pr == 0 || !ok {
		return errors.New("not running for a pull request in GitHub Actions")
	}
	runURL := fmt.Sprintf("%s/%s/%s/actions/runs/%s", os.Getenv("GITHUB_SERVER_URL"), owner, repo, os.Getenv("GITHUB_RUN_ID"))

	var report string
	if reportFile != "" {
		// A missing report means the run failed.
		data, _ := os.ReadFile(reportFile)
		report = string(data)
	}
	body := commentBody(reportFile != "", report, event.PullRequest.Head.SHA, runURL)

	client := github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	if err := prcomment.Upsert(ctx, client, owner, repo, pr, Marker, body); err != nil {
		return err
	}
	if reportFile == "" {
		return nil
	}
	resp, err := client.Issues.RemoveLabelForIssue(ctx, owner, repo, pr, Label)
	if err != nil && (resp == nil || resp.StatusCode != http.StatusNotFound) {
		return fmt.Errorf("removing the %s label: %w", Label, err)
	}
	return nil
}

func commentBody(done bool, report, sha, runURL string) string {
	sha = shortSHA(sha)
	if !done {
		return fmt.Sprintf("%s\n## Benchmark report\n\n⏳ Benchmarking %s against its merge base. Follow the progress in the [workflow run](%s).\n", Marker, sha, runURL)
	}

	body := strings.TrimSpace(report)
	if body == "" {
		body = fmt.Sprintf("%s\n## Benchmark report\n\n❌ The benchmark run for %s failed. See the [workflow run](%s) for details.", Marker, sha, runURL)
	} else if len(body) > maxCommentLen {
		body = body[:maxCommentLen] + "\n\n_Report truncated; see the workflow run for the full results._"
	}
	return fmt.Sprintf("%s\n\n<sub>[Workflow run](%s) for %s · Add the `%s` label to run the benchmarks again.</sub>\n", body, runURL, sha, Label)
}
