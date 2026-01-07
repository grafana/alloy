package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/grafana/alloy/tools/release/internal/git"
	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

func main() {
	var (
		tag        string
		footerFile string
		dryRun     bool
	)
	flag.StringVar(&tag, "tag", "", "Release tag to enrich (e.g., v1.15.0)")
	flag.StringVar(&footerFile, "footer", "", "Path to footer template file (optional)")
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run (do not update release)")
	flag.Parse()

	if tag == "" {
		log.Fatal("Release tag is required (use --tag flag)")
	}

	ctx := context.Background()

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("📝 Enriching release notes for %s\n", tag)

	// Get the release by tag
	release, err := client.GetReleaseByTag(ctx, tag)
	if err != nil {
		log.Fatalf("Failed to get release: %v", err)
	}

	releaseBody := release.GetBody()
	newBody := releaseBody

	// Add contributor information to each PR line
	newBody = addContributorInfo(ctx, client, newBody)

	// Append the release notes footer if provided
	if footerFile != "" {
		newBody, err = appendFooter(newBody, tag, footerFile)
		if err != nil {
			log.Fatalf("Failed to append footer: %v", err)
		}
	}

	if dryRun {
		fmt.Println("\n🏃 DRY RUN - No changes made")
		fmt.Println("\n--- Updated release notes ---")
		fmt.Println(newBody)
		return
	}

	// Update the release
	if err := client.UpdateReleaseBody(ctx, release.GetID(), newBody); err != nil {
		log.Fatalf("Failed to update release: %v", err)
	}

	fmt.Println("✅ Release notes updated successfully")
}

// addContributorInfo adds contributor usernames to changelog entries.
// It extracts commit SHAs from each line and looks up the author + co-authors.
func addContributorInfo(ctx context.Context, client *gh.Client, body string) string {
	lines := strings.Split(body, "\n")
	// Match commit SHA in markdown link format: "([abc1234](https://github.com/.../commit/...))"
	// This captures the short SHA from the link text
	commitPattern := regexp.MustCompile(`\(\[([a-f0-9]{7,40})\]\(https://github\.com/[^)]+\)\)\s*$`)

	for i, line := range lines {
		matches := commitPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}
		sha := matches[1]

		contributors, err := getCommitContributors(ctx, client, sha)
		if err != nil {
			fmt.Printf("⚠️  Commit %s: %v\n", sha, err)
			continue
		}
		if len(contributors) == 0 {
			fmt.Printf("   Commit %s: no human contributors found\n", sha)
			continue
		}

		fmt.Printf("   Commit %s: %v\n", sha, contributors)
		lines[i] = line + " " + formatAttribution(contributors)
	}

	return strings.Join(lines, "\n")
}

// getCommitContributors returns human contributors for a commit (author + co-authors, excluding bots).
func getCommitContributors(ctx context.Context, client *gh.Client, sha string) ([]string, error) {
	commit, err := client.GetCommit(ctx, sha)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var contributors []string

	// Add commit author (GitHub user association)
	if author := commit.GetAuthor(); author != nil {
		if username := author.GetLogin(); username != "" && !gh.IsBot(username) {
			seen[username] = true
			contributors = append(contributors, username)
		}
	}

	// Add co-authors from commit message
	for _, coauthor := range git.ParseCoAuthors(commit.GetCommit().GetMessage()) {
		username := gh.ParseUsernameFromEmail(coauthor.Email)
		if username != "" && !gh.IsBot(username) && !seen[username] {
			seen[username] = true
			contributors = append(contributors, username)
		}
	}

	return contributors, nil
}

// formatAttribution formats a list of usernames as "(@user1, @user2)".
func formatAttribution(usernames []string) string {
	var mentions []string
	for _, u := range usernames {
		mentions = append(mentions, "@"+u)
	}
	return "(" + strings.Join(mentions, ", ") + ")"
}

// appendFooter reads the release notes footer template and appends it with version substitution.
func appendFooter(body, tag, footerPath string) (string, error) {
	footer, err := os.ReadFile(footerPath)
	if err != nil {
		return "", fmt.Errorf("reading footer template: %w", err)
	}

	// Derive RELEASE_DOC_TAG from tag (e.g., v1.2.3 -> v1.2)
	releaseDocTag, err := deriveDocTag(tag)
	if err != nil {
		return "", fmt.Errorf("deriving doc tag: %w", err)
	}

	// Replace ${RELEASE_DOC_TAG} placeholder
	footerStr := strings.ReplaceAll(string(footer), "${RELEASE_DOC_TAG}", releaseDocTag)

	// Append footer to body
	return body + "\n\n" + footerStr, nil
}

// deriveDocTag derives the documentation tag from a release tag.
// e.g., "v1.2.3" -> "v1.2", "v1.2.3-rc.0" -> "v1.2"
func deriveDocTag(tag string) (string, error) {
	// Strip any prerelease suffix first (e.g., -rc.0)
	baseTag := tag
	if before, _, ok := strings.Cut(tag, "-"); ok {
		baseTag = before
	}

	// Use the version package to get major.minor
	mm, err := version.MajorMinor(baseTag)
	if err != nil {
		return "", err
	}

	// Return with v prefix
	return "v" + mm, nil
}
