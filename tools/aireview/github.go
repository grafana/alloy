package aireview

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
)

// getPRDiff fetches the diff for a pull request
func getPRDiff(ctx context.Context, client *github.Client, owner, repo string, prNumber int) (string, error) {
	opts := &github.ListOptions{PerPage: 100}
	var allFiles []string

	for {
		files, resp, err := client.PullRequests.ListFiles(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return "", fmt.Errorf("failed to list PR files: %w", err)
		}

		for _, file := range files {
			if file.Patch != nil {
				header := fmt.Sprintf("--- %s\n+++ %s\n", file.GetFilename(), file.GetFilename())
				allFiles = append(allFiles, header+*file.Patch)
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if len(allFiles) == 0 {
		return "", fmt.Errorf("no diff found for PR #%d", prNumber)
	}

	return strings.Join(allFiles, "\n\n"), nil
}
