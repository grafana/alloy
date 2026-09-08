package createrc

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"github.com/spf13/cobra"

	gh "github.com/grafana/alloy/tools/release/internal/github"
	"github.com/grafana/alloy/tools/release/internal/version"
)

const (
	mainBranch          = "main"
	releaseBranchPrefix = "release/v"
	backportLabelPrefix = "backport/v"

	// release-please names PR head branches "release-please--branches--<branch>",
	// appending "--components--<name>" for component modules.
	releasePleaseBranchPrefix = "release-please--branches--"
	releasePleaseComponentSep = "--components--"
	releasePleaseManifestPath = ".release-please-manifest.json"
)

type flags struct {
	branch string
	dryRun bool
}

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
	return info.RCNumber == 0 && info.Branch == mainBranch
}

// prereleaseParams holds parameters for creating a draft prerelease.
type prereleaseParams struct {
	Tag       string // Tag name (e.g., "v1.0.0-rc.0")
	TargetSHA string // Commit SHA to tag
	Version   string // Version string without 'v' prefix (e.g., "1.0.0")
	RCNumber  int    // Release candidate number
	PRNumber  int    // Associated release-please PR number
}

func Command() *cobra.Command {
	var flags flags

	cmd := &cobra.Command{
		Use:   "create-rc",
		Short: "Create a release candidate tag and draft prerelease for the main Alloy module",
		Long: "Creates the RC from the root Alloy module's release-please PR. Component packages " +
			"such as syntax may share that PR but are tagged separately and ignored for RCs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), flags)
		},
	}

	cmd.Flags().StringVar(&flags.branch, "branch", "", "Branch to create RC for (e.g., main or release/v1.15)")
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false, "Dry run (do not create tag or release)")
	_ = cmd.MarkFlagRequired("branch")

	return cmd
}

func run(ctx context.Context, flags flags) error {
	if flags.branch != mainBranch {
		if _, err := version.ParseReleaseBranch(flags.branch); err != nil {
			return err
		}
	}

	client, err := gh.NewClientFromEnv(ctx)
	if err != nil {
		return err
	}

	info, err := resolveRCInfo(ctx, client, flags.branch)
	if err != nil {
		return err
	}

	if flags.dryRun {
		printRCDryRun(info)
		return nil
	}

	if err := createRCRelease(ctx, client, info); err != nil {
		return err
	}

	if info.isFirstMinorRC() {
		if err := ensureBackportLabelForRC(ctx, client, info); err != nil {
			return err
		}
	}
	return nil
}

func resolveRCInfo(ctx context.Context, client *gh.Client, branch string) (rcInfo, error) {
	pr, err := findReleasePleasePR(ctx, client, branch)
	if err != nil {
		return rcInfo{}, fmt.Errorf("finding release-please PR: %w", err)
	}

	fmt.Printf("Found release-please PR #%d: %s\n", pr.GetNumber(), pr.GetTitle())
	fmt.Printf("Base branch: %s\n", pr.GetBase().GetRef())
	fmt.Printf("Head branch: %s\n", pr.GetHead().GetRef())

	baseManifest, err := getReleasePleaseManifest(ctx, client, pr.GetBase().GetSHA())
	if err != nil {
		return rcInfo{}, fmt.Errorf("reading release-please manifest from base: %w", err)
	}
	headManifest, err := getReleasePleaseManifest(ctx, client, pr.GetHead().GetSHA())
	if err != nil {
		return rcInfo{}, fmt.Errorf("reading release-please manifest from head: %w", err)
	}
	ver, err := findRootReleaseVersion(baseManifest, headManifest)
	if err != nil {
		return rcInfo{}, fmt.Errorf("determining root release version: %w", err)
	}
	fmt.Printf("Target version: %s\n", ver)

	isPatch, err := version.IsPatch(ver)
	if err != nil {
		return rcInfo{}, fmt.Errorf("parsing version: %w", err)
	}
	if branch == mainBranch && isPatch {
		mm, _ := version.MajorMinor(ver)
		return rcInfo{}, fmt.Errorf("cannot create a patch release RC from main. Use the release branch instead: --branch release/v%s", mm)
	}

	rcNumber, err := findNextRCNumber(ctx, client, ver)
	if err != nil {
		return rcInfo{}, fmt.Errorf("determining next RC number: %w", err)
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
	}, nil
}

func printRCDryRun(info rcInfo) {
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

func createRCRelease(ctx context.Context, client *gh.Client, info rcInfo) error {
	// Draft releases don't create tags until published. Create a lightweight tag (same as
	// release-please force-tag-creation) so artifact workflows run and can attach to the draft.
	if err := client.CreateTag(ctx, gh.CreateTagParams{
		Tag: info.RCTag,
		SHA: info.BranchSHA,
	}); err != nil {
		return fmt.Errorf("creating tag: %w", err)
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
		return fmt.Errorf("creating draft prerelease: %w", err)
	}
	fmt.Printf("✅ Created draft prerelease: %s\n", releaseURL)
	return nil
}

func ensureBackportLabelForRC(ctx context.Context, client *gh.Client, info rcInfo) error {
	majorMinor, err := version.MajorMinor(info.Version)
	if err != nil {
		return fmt.Errorf("parsing major.minor from version %q: %w", info.Version, err)
	}
	backportLabel := backportLabelPrefix + majorMinor
	branchName := releaseBranchPrefix + majorMinor

	created, err := client.EnsureLabel(ctx, gh.CreateLabelParams{
		Name:        backportLabel,
		Color:       gh.BackportLabelColor,
		Description: fmt.Sprintf("Backport to %s", branchName),
	})
	if err != nil {
		return fmt.Errorf("ensuring backport label: %w", err)
	}
	if created {
		fmt.Printf("✅ Created backport label: %s\n", backportLabel)
	} else {
		fmt.Printf("Backport label %s already exists\n", backportLabel)
	}
	return nil
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
	for _, pr := range prs {
		fmt.Printf("  PR #%d: %q has %d labels\n", pr.GetNumber(), pr.GetTitle(), len(pr.Labels))
		for _, label := range pr.Labels {
			fmt.Printf("    - label: %q\n", label.GetName())
		}
	}

	return selectMainModuleReleasePR(prs, baseBranch)
}

// selectMainModuleReleasePR picks the main Alloy release-please PR. With a single
// combined release PR, the head branch has no "--components--" segment. We still
// skip any leftover component-only branches. prs are already filtered to
// baseBranch by the caller.
func selectMainModuleReleasePR(prs []*github.PullRequest, baseBranch string) (*github.PullRequest, error) {
	for _, pr := range prs {
		head := pr.GetHead().GetRef()
		if !strings.HasPrefix(head, releasePleaseBranchPrefix) || strings.Contains(head, releasePleaseComponentSep) {
			continue
		}
		for _, label := range pr.Labels {
			labelName := label.GetName()
			if labelName == "autorelease: pending" || labelName == "autorelease:pending" {
				return pr, nil
			}
		}
	}

	return nil, fmt.Errorf("no main-module release-please PR found for branch %s (looked for a %q head branch with an 'autorelease: pending' label)", baseBranch, releasePleaseBranchPrefix+baseBranch)
}

func getReleasePleaseManifest(ctx context.Context, client *gh.Client, ref string) ([]byte, error) {
	file, _, _, err := client.API().Repositories.GetContents(
		ctx,
		client.Owner(),
		client.Repo(),
		releasePleaseManifestPath,
		&github.RepositoryContentGetOptions{Ref: ref},
	)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("%s is not a file", releasePleaseManifestPath)
	}
	content, err := file.GetContent()
	if err != nil {
		return nil, err
	}
	return []byte(content), nil
}

func findRootReleaseVersion(baseManifest, headManifest []byte) (string, error) {
	var baseVersions, headVersions map[string]string
	if err := json.Unmarshal(baseManifest, &baseVersions); err != nil {
		return "", fmt.Errorf("parsing base manifest: %w", err)
	}
	if err := json.Unmarshal(headManifest, &headVersions); err != nil {
		return "", fmt.Errorf("parsing head manifest: %w", err)
	}

	baseVersion, ok := baseVersions["."]
	if !ok || baseVersion == "" {
		return "", fmt.Errorf("base manifest has no root package version")
	}
	headVersion, ok := headVersions["."]
	if !ok || headVersion == "" {
		return "", fmt.Errorf("head manifest has no root package version")
	}
	if baseVersion == headVersion {
		return "", fmt.Errorf("release PR does not update the root package")
	}
	return headVersion, nil
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
