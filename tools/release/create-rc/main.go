package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"

	"github.com/google/go-github/v57/github"

	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

const (
	backportLabelPrefix = "backport/v"
	releaseBranchPrefix = "release/v"
)

// rcInfo holds the resolved parameters for creating a release candidate.
type rcInfo struct {
	PR        *github.PullRequest
	Version   string
	RCNumber  int
	RCTag     string
	BranchSHA string
	Branch    string
}

func (info rcInfo) isFirstMinorRC() bool {
	return info.RCNumber == 0 && info.Branch == "main"
}

// prereleaseParams holds parameters for creating a draft prerelease.
type prereleaseParams struct {
	Tag       string // Tag name (e.g., "v1.0.0-rc.0")
	TargetSHA string // Commit SHA to tag
	Version   string // Version string without 'v' prefix (e.g., "1.0.0")
	RCNumber  int    // Release candidate number
	PRNumber  int    // Associated release-please PR number
}

func main() {
	branch, dryRun := parseFlags()

	ctx := context.Background()
	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		log.Fatal(err)
	}

	info := resolveRCInfo(ctx, client, branch)

	if dryRun {
		printDryRun(info)
		return
	}

	createRCRelease(ctx, client, info)

	if info.isFirstMinorRC() {
		ensureBackportLabelForRC(ctx, client, info)
	}
}

func parseFlags() (string, bool) {
	var (
		dryRun bool
		branch string
	)
	flag.BoolVar(&dryRun, "dry-run", false, "Dry run (do not create tag or release)")
	flag.StringVar(&branch, "branch", "", "Branch to create RC for (e.g., main or release/v1.15)")
	flag.Parse()

	if branch == "" {
		log.Fatal("Branch is required (use --branch flag, e.g., --branch main)")
	}
	if branch != "main" {
		if _, err := version.ParseReleaseBranch(branch); err != nil {
			log.Fatal(err)
		}
	}
	return branch, dryRun
}

func resolveRCInfo(ctx context.Context, client *gh.Client, branch string) rcInfo {
	pr, err := findReleasePleasePR(ctx, client, branch)
	if err != nil {
		log.Fatalf("Failed to find release-please PR: %v", err)
	}

	fmt.Printf("Found release-please PR #%d: %s\n", pr.GetNumber(), pr.GetTitle())
	fmt.Printf("Base branch: %s\n", pr.GetBase().GetRef())
	fmt.Printf("Head branch: %s\n", pr.GetHead().GetRef())

	ver, err := extractVersionFromTitle(pr.GetTitle())
	if err != nil {
		log.Fatalf("Failed to extract version from PR title: %v", err)
	}
	fmt.Printf("Target version: %s\n", ver)

	isPatch, err := version.IsPatch(ver)
	if err != nil {
		log.Fatalf("Failed to parse version: %v", err)
	}
	if branch == "main" && isPatch {
		mm, _ := version.MajorMinor(ver)
		log.Fatalf("Cannot create a patch release RC from main. Use the release branch instead: --branch release/v%s", mm)
	}

	rcNumber, err := findNextRCNumber(ctx, client, ver)
	if err != nil {
		log.Fatalf("Failed to determine next RC number: %v", err)
	}

	rcTag := fmt.Sprintf("v%s-rc.%d", ver, rcNumber)
	fmt.Printf("Next RC tag: %s\n", rcTag)

	branchSHA := pr.GetHead().GetSHA()
	fmt.Printf("Branch HEAD SHA: %s\n", branchSHA)

	return rcInfo{
		PR:        pr,
		Version:   ver,
		RCNumber:  rcNumber,
		RCTag:     rcTag,
		BranchSHA: branchSHA,
		Branch:    branch,
	}
}

func printDryRun(info rcInfo) {
	fmt.Println("\n🏃 DRY RUN - No changes made")
	fmt.Printf("Would create tag: %s\n", info.RCTag)
	fmt.Printf("Base branch: %s\n", info.Branch)
	fmt.Printf("Release-please branch: %s\n", info.PR.GetHead().GetRef())
	fmt.Printf("Head commit: %s\n", info.BranchSHA)
	if info.isFirstMinorRC() {
		majorMinor, _ := version.MajorMinor(info.Version)
		fmt.Printf("Would ensure backport label exists: %s\n", backportLabelPrefix+majorMinor)
	}
}

func createRCRelease(ctx context.Context, client *gh.Client, info rcInfo) {
	// Draft releases don't create tags until published. Tag creation is what triggers artifacts to
	// build and get attached to releases. So we create a tag here like how release-please does with
	// force-tag-creation.
	if err := client.CreateTag(ctx, gh.CreateTagParams{
		Tag:     info.RCTag,
		SHA:     info.BranchSHA,
		Message: fmt.Sprintf("Release candidate %s", info.RCTag),
	}); err != nil {
		log.Fatalf("Failed to create tag: %v", err)
	}
	fmt.Printf("Created tag: %s -> %s\n", info.RCTag, info.BranchSHA[:12])

	releaseURL, err := createDraftPrerelease(ctx, client, prereleaseParams{
		Tag:       info.RCTag,
		TargetSHA: info.BranchSHA,
		Version:   info.Version,
		RCNumber:  info.RCNumber,
		PRNumber:  info.PR.GetNumber(),
	})
	if err != nil {
		log.Fatalf("Failed to create draft prerelease: %v", err)
	}
	fmt.Printf("✅ Created draft prerelease: %s\n", releaseURL)
}

func ensureBackportLabelForRC(ctx context.Context, client *gh.Client, info rcInfo) {
	majorMinor, err := version.MajorMinor(info.Version)
	if err != nil {
		log.Fatalf("Failed to parse major.minor from version %q: %v", info.Version, err)
	}
	backportLabel := backportLabelPrefix + majorMinor
	branchName := releaseBranchPrefix + majorMinor

	created, err := client.EnsureLabel(ctx, gh.CreateLabelParams{
		Name:        backportLabel,
		Color:       gh.BackportLabelColor,
		Description: fmt.Sprintf("Backport to %s", branchName),
	})
	if err != nil {
		log.Fatalf("Failed to ensure backport label: %v", err)
	}
	if created {
		fmt.Printf("✅ Created backport label: %s\n", backportLabel)
	} else {
		fmt.Printf("Backport label %s already exists\n", backportLabel)
	}
}

func findReleasePleasePR(ctx context.Context, client *gh.Client, baseBranch string) (*github.PullRequest, error) {
	opts := &github.PullRequestListOptions{
		State: "open",
		Base:  baseBranch,
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	prs, _, err := client.API().PullRequests.List(ctx, client.Owner(), client.Repo(), opts)
	if err != nil {
		return nil, fmt.Errorf("listing pull requests: %w", err)
	}

	fmt.Printf("Found %d open PRs targeting %s\n", len(prs), baseBranch)

	// Look for PR with "autorelease: pending" label (handle both with/without space after colon)
	for _, pr := range prs {
		fmt.Printf("  PR #%d: %q has %d labels\n", pr.GetNumber(), pr.GetTitle(), len(pr.Labels))
		for _, label := range pr.Labels {
			labelName := label.GetName()
			fmt.Printf("    - label: %q\n", labelName)
			if labelName == "autorelease: pending" || labelName == "autorelease:pending" {
				return pr, nil
			}
		}
	}

	// Fallback: look for PR with release-please title pattern
	titlePattern := regexp.MustCompile(fmt.Sprintf(`^chore\(%s\): Release`, regexp.QuoteMeta(baseBranch)))
	for _, pr := range prs {
		if titlePattern.MatchString(pr.GetTitle()) {
			return pr, nil
		}
	}

	return nil, fmt.Errorf("no release-please PR found for branch %s (looked for 'autorelease: pending' or 'autorelease:pending' label or release-please title pattern)", baseBranch)
}

func extractVersionFromTitle(title string) (string, error) {
	pattern := regexp.MustCompile(`Release\s+(\d+\.\d+\.\d+)`)
	matches := pattern.FindStringSubmatch(title)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not extract version from title: %s", title)
	}
	return matches[1], nil
}

func findNextRCNumber(ctx context.Context, client *gh.Client, ver string) (int, error) {
	opts := &github.ListOptions{PerPage: 100}
	var allTags []*github.RepositoryTag

	for {
		tags, resp, err := client.API().Repositories.ListTags(ctx, client.Owner(), client.Repo(), opts)
		if err != nil {
			return 0, fmt.Errorf("listing tags: %w", err)
		}
		allTags = append(allTags, tags...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	// Find existing RC tags for this version
	rcPattern := regexp.MustCompile(fmt.Sprintf(`^v%s-rc\.(\d+)$`, regexp.QuoteMeta(ver)))
	var rcNumbers []int

	for _, tag := range allTags {
		matches := rcPattern.FindStringSubmatch(tag.GetName())
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			rcNumbers = append(rcNumbers, num)
		}
	}

	if len(rcNumbers) == 0 {
		return 0, nil
	}

	sort.Ints(rcNumbers)
	return rcNumbers[len(rcNumbers)-1] + 1, nil
}

func createDraftPrerelease(ctx context.Context, client *gh.Client, p prereleaseParams) (string, error) {
	body := fmt.Sprintf(`## Release Candidate %d for v%s

This is a **release candidate** and should be used for testing purposes only.

**⚠️ This is a pre-release. Do not use in production.**

### Changes

See the [release PR #%d](https://github.com/%s/%s/pull/%d) for the full changelog.
`, p.RCNumber, p.Version, p.PRNumber, client.Owner(), client.Repo(), p.PRNumber)

	release := &github.RepositoryRelease{
		TagName:         github.String(p.Tag),
		TargetCommitish: github.String(p.TargetSHA),
		Name:            github.String(p.Tag),
		Body:            github.String(body),
		Draft:           github.Bool(true),
		Prerelease:      github.Bool(true),
	}

	created, _, err := client.API().Repositories.CreateRelease(ctx, client.Owner(), client.Repo(), release)
	if err != nil {
		return "", fmt.Errorf("creating release: %w", err)
	}

	return created.GetHTMLURL(), nil
}
