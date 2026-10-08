package prcomment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v57/github"
)

// Upsert updates the pull request comment that contains marker, or creates one
// if there is none.
func Upsert(ctx context.Context, client *github.Client, owner, repo string, prNumber int, marker, body string) error {
	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var existingComment *github.IssueComment

	// Find existing comment with the marker
	for {
		comments, resp, err := client.Issues.ListComments(ctx, owner, repo, prNumber, opts)
		if err != nil {
			return fmt.Errorf("failed to list comments: %w", err)
		}

		for _, comment := range comments {
			if strings.Contains(comment.GetBody(), marker) {
				existingComment = comment
				break
			}
		}

		if existingComment != nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if existingComment != nil {
		// Update existing comment
		_, _, err := client.Issues.EditComment(ctx, owner, repo, existingComment.GetID(), &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("failed to update comment: %w", err)
		}
		fmt.Printf("Updated existing comment (ID: %d)\n", existingComment.GetID())
	} else {
		// Create new comment
		_, _, err := client.Issues.CreateComment(ctx, owner, repo, prNumber, &github.IssueComment{
			Body: &body,
		})
		if err != nil {
			return fmt.Errorf("failed to create comment: %w", err)
		}
		fmt.Println("Created new comment")
	}

	return nil
}
