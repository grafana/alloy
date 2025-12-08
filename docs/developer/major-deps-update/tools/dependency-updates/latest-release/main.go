package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// Constants
// ============================================================================

// LookupMethod defines how to fetch version information
type LookupMethod string

const (
	GoModule      LookupMethod = "gomodule" // Use Go module versions
	GitHubRelease LookupMethod = "release"  // Use GitHub releases
	GitTag        LookupMethod = "tag"      // Use Git tags
)

var specialModuleMappings = map[string]string{
	"go.opentelemetry.io/otel":          "open-telemetry/opentelemetry-go",
	"go.opentelemetry.io/contrib":       "open-telemetry/opentelemetry-go-contrib",
	"go.opentelemetry.io/collector":     "open-telemetry/opentelemetry-collector",
	"go.opentelemetry.io/build-tools":   "open-telemetry/opentelemetry-go-build-tools",
	"go.opentelemetry.io/auto":          "open-telemetry/opentelemetry-go-instrumentation",
	"go.opentelemetry.io/obi":           "grafana/opentelemetry-ebpf-instrumentation",
	"go.opentelemetry.io/ebpf-profiler": "grafana/opentelemetry-ebpf-profiler",
}

var specialVersioningRepos = []string{"prometheus/prometheus"}

// primaryLookupMethod maps GitHub repositories to their primary lookup method
// This determines which single source is used for each dependency
var primaryLookupMethod = map[string]LookupMethod{
	// Grafana forks use Git tags as primary source
	"grafana/opentelemetry-ebpf-profiler":        GitTag,
	"grafana/opentelemetry-ebpf-instrumentation": GitTag,

	// Major dependencies use GitHub releases as primary source
	"prometheus/prometheus":                          GitHubRelease,
	"prometheus/common":                              GitHubRelease,
	"prometheus/client_golang":                       GitHubRelease,
	"prometheus/client_model":                        GitHubRelease,
	"open-telemetry/opentelemetry-collector":         GitHubRelease,
	"open-telemetry/opentelemetry-collector-contrib": GitHubRelease,
	"open-telemetry/opentelemetry-go":                GitHubRelease,
	"open-telemetry/opentelemetry-go-contrib":        GitHubRelease,
	"grafana/beyla":                                  GitHubRelease,
	"grafana/loki":                                   GitHubRelease,
}

// ============================================================================
// Parsing Functions
// ============================================================================

// Release represents a GitHub release
type Release struct {
	Tag       string
	Title     string
	Published string
}

// VersionInfo represents a Go module version with its publish time
type VersionInfo struct {
	Version string
	Time    time.Time
}

// goListModuleJSON represents the JSON output from 'go list -m -json'
type goListModuleJSON struct {
	Path     string    `json:"Path"`
	Version  string    `json:"Version"`
	Time     time.Time `json:"Time"`
	Versions []string  `json:"Versions"`
}

// parseGoVersionsOutput parses the output of 'go list -m -versions' command.
// It mirrors:
//
//	go list -m -versions <module> | tr ' ' '\n' | grep -v '+incompatible'
//
// by skipping the module path and filtering out any versions containing
// "+incompatible".
func parseGoVersionsOutput(output string) []string {
	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) <= 1 {
		return []string{}
	}

	versions := make([]string, 0, len(parts)-1)
	for _, v := range parts[1:] { // Skip module path
		if strings.Contains(v, "+incompatible") {
			continue
		}
		versions = append(versions, v)
	}
	return versions
}

// getGoModuleVersions gets all versions of a Go module with their publish times,
// sorted by publish date (newest first)
func getGoModuleVersions(modulePath string) ([]VersionInfo, error) {
	// First, get all available versions
	cmd := exec.Command("go", "list", "-m", "-json", "-versions", modulePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error fetching Go module versions: %w", err)
	}

	// Parse the JSON to get the list of versions
	var modInfo goListModuleJSON
	if err := json.Unmarshal(output, &modInfo); err != nil {
		return nil, fmt.Errorf("error parsing module info: %w", err)
	}

	// Filter out incompatible versions
	versions := make([]string, 0, len(modInfo.Versions))
	for _, v := range modInfo.Versions {
		if !strings.Contains(v, "+incompatible") {
			versions = append(versions, v)
		}
	}

	if len(versions) == 0 {
		return []VersionInfo{}, nil
	}

	// Fetch publish time for each version
	versionInfos := make([]VersionInfo, 0, len(versions))
	for _, version := range versions {
		cmd := exec.Command("go", "list", "-m", "-json", modulePath+"@"+version)
		output, err := cmd.Output()
		if err != nil {
			// If we can't get the time for this version, skip it
			continue
		}

		var vInfo goListModuleJSON
		if err := json.Unmarshal(output, &vInfo); err != nil {
			continue
		}

		versionInfos = append(versionInfos, VersionInfo{
			Version: version,
			Time:    vInfo.Time,
		})
	}

	// Sort by time, newest first
	sort.Slice(versionInfos, func(i, j int) bool {
		return versionInfos[i].Time.After(versionInfos[j].Time)
	})

	return versionInfos, nil
}

// extractGitHubRepo extracts GitHub owner/repo from a Go module path
func extractGitHubRepo(modulePath string) string {
	// Check special mappings first (e.g., OpenTelemetry modules)
	for prefix, repo := range specialModuleMappings {
		if strings.HasPrefix(modulePath, prefix) {
			return repo
		}
	}

	// Standard GitHub pattern: github.com/owner/repo
	re := regexp.MustCompile(`github\.com/([^/]+/[^/]+)`)
	matches := re.FindStringSubmatch(modulePath)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// parseGitHubReleasesOutput parses the output of 'gh release list' command
func parseGitHubReleasesOutput(output string) []Release {
	var releases []Release
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 3 {
			title := strings.TrimSpace(parts[0])
			releaseType := strings.TrimSpace(parts[1])
			tag := strings.TrimSpace(parts[2])
			published := "N/A"
			if len(parts) >= 4 {
				published = strings.TrimSpace(parts[3])
			}

			// Use tag as the version, and combine type with title for display
			displayTitle := title
			if releaseType != "" {
				displayTitle = releaseType + " - " + title
			}

			releases = append(releases, Release{
				Tag:       tag,
				Title:     displayTitle,
				Published: published,
			})
		}
	}
	return releases
}

// getGitHubReleases gets GitHub releases using gh CLI
func getGitHubReleases(repo string, limit int) ([]Release, error) {
	cmd := exec.Command("gh", "release", "list", "-R", repo, "-L", strconv.Itoa(limit))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error fetching GitHub releases: %w", err)
	}
	return parseGitHubReleasesOutput(string(output)), nil
}

// parseGitTagsOutput parses the output of 'gh api repos/OWNER/REPO/tags' command
func parseGitTagsOutput(output string) []Release {
	var releases []Release
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 1 {
			tag := strings.TrimSpace(parts[0])
			published := "N/A"
			if len(parts) >= 2 {
				published = strings.TrimSpace(parts[1])
			}

			releases = append(releases, Release{
				Tag:       tag,
				Title:     tag, // For tags, use the tag name as the title
				Published: published,
			})
		}
	}
	return releases
}

// getGitTags gets Git tags using gh API
func getGitTags(repo string, limit int) ([]Release, error) {
	// Use per_page to limit results
	perPage := limit
	if perPage > 100 {
		perPage = 100
	}

	cmd := exec.Command("gh", "api", "repos/"+repo+"/tags",
		"--jq", `.[] | "\(.name)\t\(if .commit.commit.author.date then .commit.commit.author.date else "N/A" end)"`,
		"-X", "GET",
		"-F", fmt.Sprintf("per_page=%d", perPage))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error fetching Git tags: %w", err)
	}

	return parseGitTagsOutput(string(output)), nil
}

// ============================================================================
// Version Conversion Functions (Prometheus-style)
// ============================================================================

// isSemverTag reports whether a tag looks like a semantic version starting
// with a "v" prefix, e.g. v1.2.3, v3.6.2-rc.1, etc.
func isSemverTag(tag string) bool {
	re := regexp.MustCompile(`^v\d+\.\d+\.\d+.*$`)
	return re.MatchString(tag)
}

// latestSemverTag scans releases (assumed newest-first as returned by
// "gh release list") and returns the first tag that looks like a semantic
// version (vMAJOR.MINOR.PATCH...). If none is found, it returns an empty
// string.
func latestSemverTag(releases []Release) string {
	for _, r := range releases {
		if isSemverTag(r.Tag) {
			return r.Tag
		}
	}
	return ""
}

// normalizeVersion normalizes version for comparison (ensure 'v' prefix)
func normalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

// latestGoModuleVersion picks the most relevant "latest" Go module version from
// the list (already sorted by publish date, newest first).
//
// For Prometheus-style repos (specialVersioning == true) we want the latest
// v0.x.y version so that it can be mapped to the corresponding GitHub release
// (vMAJOR.MINOR.PATCH).
//
// For other repos we prefer the latest semantic version (vMAJOR.MINOR.PATCH...)
// and intentionally skip versions that are clearly metadata-only markers such
// as "*-retract".
func latestGoModuleVersion(goVersions []VersionInfo, specialVersioning bool) string {
	if len(goVersions) == 0 {
		return "N/A"
	}

	// Prometheus-style mapping: pick the newest v0.x.y version.
	if specialVersioning {
		for _, vInfo := range goVersions {
			v := vInfo.Version
			if strings.HasPrefix(v, "v0.") && !strings.Contains(v, "-retract") {
				return v
			}
		}
		// If we didn't find a v0.* version, fall back to the first entry (newest).
		return goVersions[0].Version
	}

	// Generic modules: prefer the latest semantic version that is not a retract
	// marker. Since versions are already sorted by date (newest first), we just
	// need to find the first one that matches our criteria.
	for _, vInfo := range goVersions {
		v := vInfo.Version
		if isSemverTag(v) && !strings.Contains(v, "-retract") {
			return v
		}
	}

	// Fallback: if nothing matched our expectations, use the first entry (newest).
	return goVersions[0].Version
}

// githubVersionToGoModule converts Prometheus/Loki GitHub version to Go module version
// Example: v3.4.2 -> v0.304.2
func githubVersionToGoModule(githubVersion string) string {
	re := regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(.*)$`)
	matches := re.FindStringSubmatch(githubVersion)
	if len(matches) >= 4 {
		major := matches[1]
		minor := matches[2]
		patch := matches[3]
		suffix := ""
		if len(matches) > 4 {
			suffix = matches[4]
		}

		// Go module version: v0.{MAJOR}{MINOR:02d}.PATCH
		minorInt, _ := strconv.Atoi(minor)
		goMinor := fmt.Sprintf("%s%02d", major, minorInt)
		return fmt.Sprintf("v0.%s.%s%s", goMinor, patch, suffix)
	}
	return githubVersion
}

// goModuleVersionToGitHub converts Prometheus/Loki Go module version to GitHub version
// Example: v0.304.2 -> v3.4.2
func goModuleVersionToGitHub(goVersion string) string {
	re := regexp.MustCompile(`^v0\.(\d+)\.(\d+)(.*)$`)
	matches := re.FindStringSubmatch(goVersion)
	if len(matches) >= 3 {
		combinedVersion := matches[1]
		patch := matches[2]
		suffix := ""
		if len(matches) > 3 {
			suffix = matches[3]
		}

		// Extract major (first 1-2 digits) and minor (last 2 digits)
		if len(combinedVersion) >= 3 {
			major := combinedVersion[:len(combinedVersion)-2]
			minor := combinedVersion[len(combinedVersion)-2:]
			minorInt, _ := strconv.Atoi(minor)
			return fmt.Sprintf("v%s.%d.%s%s", major, minorInt, patch, suffix)
		}
	}
	return goVersion
}

// usesSpecialVersioning checks if the repository uses special versioning (Prometheus style)
func usesSpecialVersioning(repo string) bool {
	for _, r := range specialVersioningRepos {
		if r == repo {
			return true
		}
	}
	return false
}

// ============================================================================
// Lookup Method Functions
// ============================================================================

// getPrimaryLookupMethod determines which single method to use for fetching version information
// It checks explicit mappings first, then pattern matching, then defaults
func getPrimaryLookupMethod(modulePath, githubRepo string) LookupMethod {
	// Check explicit mapping first
	if method, exists := primaryLookupMethod[githubRepo]; exists {
		return method
	}

	// Pattern matching: GitHub repos default to GitHub releases
	if strings.HasPrefix(modulePath, "github.com/") {
		return GitHubRelease
	}

	// Default to Go modules for non-GitHub modules (e.g., golang.org/x/*, gopkg.in/*)
	return GoModule
}

// ============================================================================
// Display Functions
// ============================================================================

func displayLatestVersion(latestVersion string, method LookupMethod, specialVersioning bool) {
	methodName := "Go modules"
	if method == GitHubRelease {
		methodName = "GitHub releases"
	} else if method == GitTag {
		methodName = "Git tags"
	}

	fmt.Printf("Lookup method: %s\n", methodName)

	if specialVersioning && latestVersion != "N/A" {
		// For Prometheus-style repos, show both formats
		if method == GitHubRelease || method == GitTag {
			// We have GitHub version, show Go module equivalent
			goModVer := githubVersionToGoModule(latestVersion)
			fmt.Printf("Latest version:  %s (Go module: %s)\n", latestVersion, goModVer)
		} else {
			// We have Go module version, show GitHub equivalent
			githubVer := goModuleVersionToGitHub(latestVersion)
			fmt.Printf("Latest version:  %s (GitHub: %s)\n", latestVersion, githubVer)
		}
	} else {
		fmt.Printf("Latest version:  %s\n", latestVersion)
	}
}

func displayVersionList(goVersions []VersionInfo, releases []Release, method LookupMethod, specialVersioning bool) {
	if method == GoModule {
		displayGoVersions(goVersions, specialVersioning)
	} else {
		displayGitHubReleases(releases, specialVersioning)
	}
}

func displayGoVersions(goVersions []VersionInfo, specialVersioning bool) {
	if len(goVersions) == 0 {
		fmt.Println("\nNo versions found.")
		return
	}

	fmt.Println("\nRecent versions (last 10, newest first):")
	// Already sorted by date (newest first), just take first 10
	limit := 10
	if len(goVersions) < limit {
		limit = len(goVersions)
	}

	for i := 0; i < limit; i++ {
		vInfo := goVersions[i]
		dateDisplay := vInfo.Time.Format("2006-01-02")

		if specialVersioning {
			githubVer := goModuleVersionToGitHub(vInfo.Version)
			fmt.Printf("  %-15s %-12s (GitHub: %s)\n", vInfo.Version, dateDisplay, githubVer)
		} else {
			fmt.Printf("  %-15s %-12s\n", vInfo.Version, dateDisplay)
		}
	}
}

func displayGitHubReleases(releases []Release, specialVersioning bool) {
	if len(releases) == 0 {
		fmt.Println("\nNo versions found.")
		return
	}

	fmt.Println("\nRecent versions (last 10, newest first):")
	limit := 10
	if len(releases) < limit {
		limit = len(releases)
	}

	for i := 0; i < limit; i++ {
		release := releases[i]
		dateDisplay := release.Published
		if strings.Contains(dateDisplay, "T") {
			dateDisplay = strings.Split(dateDisplay, "T")[0]
		}

		titleDisplay := release.Title
		if len(titleDisplay) > 40 {
			titleDisplay = titleDisplay[:37] + "..."
		}

		if specialVersioning {
			goVer := githubVersionToGoModule(release.Tag)
			fmt.Printf("  %-15s %-12s (Go: %-12s) %s\n", release.Tag, dateDisplay, goVer, titleDisplay)
		} else {
			fmt.Printf("  %-15s %-12s %s\n", release.Tag, dateDisplay, titleDisplay)
		}
	}
}

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	limit := flag.Int("limit", 20, "Number of recent GitHub releases to fetch (default: 20)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Error: module_path is required")
		fmt.Fprintln(os.Stderr, "\nUsage: go run main.go [flags] <module_path>")
		fmt.Fprintln(os.Stderr, "  module_path: The Go module path (e.g., github.com/prometheus/common)")
		flag.PrintDefaults()
		os.Exit(1)
	}

	modulePath := flag.Arg(0)

	fmt.Printf("Go Module: %s\n\n", modulePath)

	// Extract GitHub repo (if applicable)
	githubRepo := extractGitHubRepo(modulePath)

	// Determine the single lookup method to use
	lookupMethod := getPrimaryLookupMethod(modulePath, githubRepo)

	// Determine if this repo uses special versioning (Prometheus-style)
	specialVersioning := githubRepo != "" && usesSpecialVersioning(githubRepo)

	// Perform ONLY ONE lookup based on the primary method
	var versions []VersionInfo
	var releases []Release
	var latestVersion string
	var err error

	switch lookupMethod {
	case GoModule:
		versions, err = getGoModuleVersions(modulePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching Go module versions: %v\n", err)
			if !strings.Contains(modulePath, ".") {
				fmt.Fprintf(os.Stderr, "\nHint: %q does not look like a full Go module path.\n", modulePath)
				fmt.Fprintln(os.Stderr, "      Please provide the full module path, for example: github.com/prometheus/common")
			}
			os.Exit(1)
		}
		latestVersion = latestGoModuleVersion(versions, specialVersioning)

	case GitHubRelease:
		if githubRepo == "" {
			fmt.Fprintln(os.Stderr, "Error: Could not extract GitHub repository from module path.")
			os.Exit(1)
		}
		releases, err = getGitHubReleases(githubRepo, *limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching GitHub releases: %v\n", err)
			os.Exit(1)
		}
		if len(releases) > 0 {
			if tag := latestSemverTag(releases); tag != "" {
				latestVersion = tag
			} else {
				latestVersion = releases[0].Tag
			}
		} else {
			latestVersion = "N/A"
		}

	case GitTag:
		if githubRepo == "" {
			fmt.Fprintln(os.Stderr, "Error: Could not extract GitHub repository from module path.")
			os.Exit(1)
		}
		releases, err = getGitTags(githubRepo, *limit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching Git tags: %v\n", err)
			os.Exit(1)
		}
		if len(releases) > 0 {
			latestVersion = releases[0].Tag
		} else {
			latestVersion = "N/A"
		}
	}

	// Display results
	displayLatestVersion(latestVersion, lookupMethod, specialVersioning)
	displayVersionList(versions, releases, lookupMethod, specialVersioning)
}
