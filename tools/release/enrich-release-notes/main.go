package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

func main() {
	var (
		tag    string
		dryRun bool
	)
	flag.StringVar(&tag, "tag", "", "Release tag to enrich (e.g., v1.15.0)")
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

	// Append the release notes footer
	newBody, err = appendFooter(newBody, tag)
	if err != nil {
		log.Fatalf("Failed to append footer: %v", err)
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

// addContributorInfo adds contributor usernames to PR references in the release notes.
func addContributorInfo(ctx context.Context, client *gh.Client, body string) string {
	lines := strings.Split(body, "\n")
	prPattern := regexp.MustCompile(`\[#(\d+)\]\([^)]+\)`)

	for i, line := range lines {
		matches := prPattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		prNumber := 0
		if _, err := fmt.Sscanf(matches[1], "%d", &prNumber); err != nil {
			continue
		}

		pr, err := client.GetPR(ctx, prNumber)
		if err != nil {
			fmt.Printf("⚠️  Failed to get PR #%d: %v\n", prNumber, err)
			continue
		}

		username := pr.GetUser().GetLogin()
		fmt.Printf("   PR #%d authored by @%s\n", prNumber, username)

		// Append username to the line if not already present
		attribution := fmt.Sprintf("(@%s)", username)
		if !strings.Contains(line, attribution) {
			lines[i] = line + " " + attribution
		}
	}

	return strings.Join(lines, "\n")
}

// appendFooter reads the release notes footer template and appends it with version substitution.
func appendFooter(body, tag string) (string, error) {
	// Read footer template - path is relative to tools directory (where go run is executed from)
	footerPath := "release/release-notes-footer.md"
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
	if idx := strings.Index(tag, "-"); idx != -1 {
		baseTag = tag[:idx]
	}

	// Use the version package to get major.minor
	mm, err := version.MajorMinor(baseTag)
	if err != nil {
		return "", err
	}

	// Return with v prefix
	return "v" + mm, nil
}
