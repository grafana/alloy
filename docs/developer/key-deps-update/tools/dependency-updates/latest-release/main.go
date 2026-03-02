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

type LookupMethod string

const (
	GoModule      LookupMethod = "gomodule"
	GitHubRelease LookupMethod = "release"
	GitTag        LookupMethod = "tag"
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

var primaryLookupMethod = map[string]LookupMethod{
	"grafana/opentelemetry-ebpf-profiler":        GitTag,
	"grafana/opentelemetry-ebpf-instrumentation": GitTag,

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

type Release struct {
	Tag       string
	Title     string
	Published string
}

type VersionInfo struct {
	Version string
	Time    time.Time
}

type goListModuleJSON struct {
	Path     string    `json:"Path"`
	Version  string    `json:"Version"`
	Time     time.Time `json:"Time"`
	Versions []string  `json:"Versions"`
}

func parseGoVersionsOutput(output string) []string {
	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) <= 1 {
		return []string{}
	}

	versions := make([]string, 0, len(parts)-1)
	for _, v := range parts[1:] {
		if !strings.Contains(v, "+incompatible") {
			versions = append(versions, v)
		}
	}
	return versions
}

func getGoModuleVersions(modulePath string) ([]VersionInfo, error) {
	cmd := exec.Command("go", "list", "-m", "-json", "-versions", modulePath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error fetching Go module versions: %w", err)
	}

	var modInfo goListModuleJSON
	if err := json.Unmarshal(output, &modInfo); err != nil {
		return nil, fmt.Errorf("error parsing module info: %w", err)
	}

	versions := make([]string, 0, len(modInfo.Versions))
	for _, v := range modInfo.Versions {
		if !strings.Contains(v, "+incompatible") {
			versions = append(versions, v)
		}
	}

	if len(versions) == 0 {
		return []VersionInfo{}, nil
	}

	versionInfos := make([]VersionInfo, 0, len(versions))
	for _, version := range versions {
		cmd := exec.Command("go", "list", "-m", "-json", modulePath+"@"+version)
		output, err := cmd.Output()
		if err != nil {
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

	sort.Slice(versionInfos, func(i, j int) bool {
		return versionInfos[i].Time.After(versionInfos[j].Time)
	})

	return versionInfos, nil
}

func extractGitHubRepo(modulePath string) string {
	for prefix, repo := range specialModuleMappings {
		if strings.HasPrefix(modulePath, prefix) {
			return repo
		}
	}

	re := regexp.MustCompile(`github\.com/([^/]+/[^/]+)`)
	matches := re.FindStringSubmatch(modulePath)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

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

func getGitHubReleases(repo string, limit int) ([]Release, error) {
	cmd := exec.Command("gh", "release", "list", "-R", repo, "-L", strconv.Itoa(limit))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("error fetching GitHub releases: %w", err)
	}
	return parseGitHubReleasesOutput(string(output)), nil
}

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
				Title:     tag,
				Published: published,
			})
		}
	}
	return releases
}

func getGitTags(repo string, limit int) ([]Release, error) {
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

func isSemverTag(tag string) bool {
	re := regexp.MustCompile(`^v\d+\.\d+\.\d+.*$`)
	return re.MatchString(tag)
}

func latestSemverTag(releases []Release) string {
	for _, r := range releases {
		if isSemverTag(r.Tag) {
			return r.Tag
		}
	}
	return ""
}

func normalizeVersion(version string) string {
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func latestGoModuleVersion(goVersions []VersionInfo) string {
	if len(goVersions) == 0 {
		return "N/A"
	}

	for _, vInfo := range goVersions {
		v := vInfo.Version
		if isSemverTag(v) && !strings.Contains(v, "-retract") {
			return v
		}
	}

	return goVersions[0].Version
}

func getPrimaryLookupMethod(modulePath, githubRepo string) LookupMethod {
	if method, exists := primaryLookupMethod[githubRepo]; exists {
		return method
	}

	if strings.HasPrefix(modulePath, "github.com/") {
		return GitHubRelease
	}

	return GoModule
}

func displayLatestVersion(latestVersion string, method LookupMethod) {
	var methodName string
	switch method {
	case GitHubRelease:
		methodName = "GitHub releases"
	case GitTag:
		methodName = "Git tags"
	default:
		methodName = "Go modules"
	}

	fmt.Printf("Lookup method: %s\n", methodName)
	fmt.Printf("Latest version:  %s\n", latestVersion)
}

func displayVersionList(goVersions []VersionInfo, releases []Release, method LookupMethod) {
	if method == GoModule {
		displayGoVersions(goVersions)
	} else {
		displayGitHubReleases(releases)
	}
}

func displayGoVersions(goVersions []VersionInfo) {
	if len(goVersions) == 0 {
		fmt.Println("\nNo versions found.")
		return
	}

	fmt.Println("\nRecent versions (last 10, newest first):")
	limit := 10
	if len(goVersions) < limit {
		limit = len(goVersions)
	}

	for i := 0; i < limit; i++ {
		vInfo := goVersions[i]
		dateDisplay := vInfo.Time.Format("2006-01-02")
		fmt.Printf("  %-15s %-12s\n", vInfo.Version, dateDisplay)
	}
}

func displayGitHubReleases(releases []Release) {
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

		fmt.Printf("  %-15s %-12s %s\n", release.Tag, dateDisplay, titleDisplay)
	}
}

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

	githubRepo := extractGitHubRepo(modulePath)
	lookupMethod := getPrimaryLookupMethod(modulePath, githubRepo)

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
		latestVersion = latestGoModuleVersion(versions)

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

	displayLatestVersion(latestVersion, lookupMethod)
	displayVersionList(versions, releases, lookupMethod)
}
