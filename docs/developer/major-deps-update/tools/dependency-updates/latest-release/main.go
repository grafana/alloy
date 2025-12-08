package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================================
// Constants
// ============================================================================

var specialModuleMappings = map[string]string{
	"go.opentelemetry.io/otel":          "open-telemetry/opentelemetry-go",
	"go.opentelemetry.io/contrib":       "open-telemetry/opentelemetry-go-contrib",
	"go.opentelemetry.io/collector":     "open-telemetry/opentelemetry-collector",
	"go.opentelemetry.io/build-tools":   "open-telemetry/opentelemetry-go-build-tools",
	"go.opentelemetry.io/auto":          "open-telemetry/opentelemetry-go-instrumentation",
	"go.opentelemetry.io/obi":           "open-telemetry/opentelemetry-ebpf-instrumentation",
	"go.opentelemetry.io/ebpf-profiler": "open-telemetry/opentelemetry-ebpf-profiler",
}

var specialVersioningRepos = []string{"prometheus/prometheus"}

// ============================================================================
// Parsing Functions
// ============================================================================

// Release represents a GitHub release
type Release struct {
	Tag       string
	Title     string
	Published string
}

// parseGoVersionsOutput parses the output of 'go list -m -versions' command
func parseGoVersionsOutput(output string) []string {
	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) > 1 {
		return parts[1:] // Skip module path, return versions
	}
	return []string{}
}

// getGoModuleVersions gets all versions of a Go module using go list -m -versions
func getGoModuleVersions(modulePath string) ([]string, error) {
	cmd := exec.Command("go", "list", "-m", "-versions", modulePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error fetching Go module versions: %w", err)
	}
	return parseGoVersionsOutput(string(output)), nil
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

// ============================================================================
// Version Conversion Functions (Prometheus-style)
// ============================================================================

// normalizeVersion normalizes version for comparison (ensure 'v' prefix)
func normalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
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
// Display Functions
// ============================================================================

func displayLatestVersions(latestGo, latestGitHub string, specialVersioning bool) {
	if specialVersioning && latestGo != "N/A" {
		convertedGitHub := goModuleVersionToGitHub(latestGo)
		fmt.Printf("Latest from Go modules:      %s (GitHub: %s)\n", latestGo, convertedGitHub)
	} else {
		fmt.Printf("Latest from Go modules:      %s\n", latestGo)
	}

	if specialVersioning && latestGitHub != "N/A" {
		convertedGo := githubVersionToGoModule(latestGitHub)
		fmt.Printf("Latest from GitHub releases: %s (Go module: %s)\n", latestGitHub, convertedGo)
	} else {
		fmt.Printf("Latest from GitHub releases: %s\n", latestGitHub)
	}
}

func displayVersionComparison(latestGo, latestGitHub string, specialVersioning bool) {
	if latestGo == "N/A" || latestGitHub == "N/A" {
		return
	}

	versionsMatch := false
	if specialVersioning {
		convertedGitHubVersion := githubVersionToGoModule(latestGitHub)
		versionsMatch = normalizeVersion(latestGo) == normalizeVersion(convertedGitHubVersion)
	} else {
		versionsMatch = normalizeVersion(latestGo) == normalizeVersion(latestGitHub)
	}

	if versionsMatch {
		fmt.Println("\n✓ Both sources agree on the latest version")
	} else {
		fmt.Println("\n⚠️  DISCREPANCY DETECTED: Latest versions differ between sources!")
	}
}

func displayGoVersions(goVersions []string, specialVersioning bool) {
	if len(goVersions) == 0 {
		fmt.Println("\nNo Go module versions found.")
		return
	}

	fmt.Println("\nAll Go module versions (last 10):")
	// Get last 10, newest first
	start := len(goVersions) - 10
	if start < 0 {
		start = 0
	}
	displayVersions := goVersions[start:]
	// Reverse to show newest first
	for i := len(displayVersions) - 1; i >= 0; i-- {
		version := displayVersions[i]
		if specialVersioning {
			githubVer := goModuleVersionToGitHub(version)
			fmt.Printf("  %-15s (GitHub: %s)\n", version, githubVer)
		} else {
			fmt.Printf("  %s\n", version)
		}
	}
}

func displayGitHubReleases(releases []Release, specialVersioning bool) {
	if len(releases) == 0 {
		fmt.Println("\nNo GitHub releases found.")
		return
	}

	fmt.Println("\nRecent GitHub releases:")
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

	// Fetch data from both sources
	goVersions, goErr := getGoModuleVersions(modulePath)
	if goErr != nil {
		fmt.Fprintf(os.Stderr, "Error fetching Go module versions: %v\n", goErr)
		goVersions = []string{}
	}

	githubRepo := extractGitHubRepo(modulePath)
	var githubReleases []Release

	var ghErr error
	if githubRepo != "" {
		var err error
		githubReleases, err = getGitHubReleases(githubRepo, *limit)
		if err != nil {
			ghErr = err
			fmt.Fprintf(os.Stderr, "Error fetching GitHub releases: %v\n", err)
			githubReleases = []Release{}
		}
	} else {
		fmt.Fprintln(os.Stderr, "Warning: Could not extract GitHub repository from module path.")
		fmt.Fprintln(os.Stderr, "GitHub releases will not be available.")
		githubReleases = []Release{}
	}

	// Determine versioning scheme and latest versions
	specialVersioning := githubRepo != "" && usesSpecialVersioning(githubRepo)
	latestGo := "N/A"
	if len(goVersions) > 0 {
		latestGo = goVersions[len(goVersions)-1]
	}

	latestGitHub := "N/A"
	if len(githubReleases) > 0 {
		latestGitHub = githubReleases[0].Tag
	}

	// If both data sources failed and the module path looks shortened (e.g., "prometheus/common"),
	// print a helpful suggestion about using the full module path.
	if goErr != nil && (ghErr != nil || githubRepo == "") && !strings.Contains(modulePath, ".") {
		fmt.Fprintf(os.Stderr, "\nHint: %q does not look like a full Go module path.\n", modulePath)
		fmt.Fprintln(os.Stderr, "      Please provide the full module path, for example: github.com/prometheus/common")
	}

	// Display results
	displayLatestVersions(latestGo, latestGitHub, specialVersioning)
	displayVersionComparison(latestGo, latestGitHub, specialVersioning)
	displayGoVersions(goVersions, specialVersioning)
	displayGitHubReleases(githubReleases, specialVersioning)
}
