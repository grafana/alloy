package main

import (
	"testing"
	"time"
)

func TestStandardGitHubPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.com/prometheus/common", "prometheus/common"},
		{"github.com/stretchr/testify", "stretchr/testify"},
		{"github.com/go-kit/kit", "go-kit/kit"},
	}

	for _, tt := range tests {
		result := extractGitHubRepo(tt.input)
		if result != tt.expected {
			t.Errorf("extractGitHubRepo(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGitHubPathWithSubpackage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.com/prometheus/common/model", "prometheus/common"},
		{"github.com/aws/aws-sdk-go/service/s3", "aws/aws-sdk-go"},
	}

	for _, tt := range tests {
		result := extractGitHubRepo(tt.input)
		if result != tt.expected {
			t.Errorf("extractGitHubRepo(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestOpenTelemetryMappings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"go.opentelemetry.io/otel", "open-telemetry/opentelemetry-go"},
		{"go.opentelemetry.io/otel/sdk", "open-telemetry/opentelemetry-go"},
		{"go.opentelemetry.io/contrib/otelconf", "open-telemetry/opentelemetry-go-contrib"},
		{"go.opentelemetry.io/collector", "open-telemetry/opentelemetry-collector"},
		{"go.opentelemetry.io/build-tools", "open-telemetry/opentelemetry-go-build-tools"},
		{"go.opentelemetry.io/auto", "open-telemetry/opentelemetry-go-instrumentation"},
	}

	for _, tt := range tests {
		result := extractGitHubRepo(tt.input)
		if result != tt.expected {
			t.Errorf("extractGitHubRepo(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNonGitHubPath(t *testing.T) {
	tests := []string{
		"golang.org/x/text",
		"gopkg.in/yaml.v3",
		"example.com/foo/bar",
	}

	for _, input := range tests {
		result := extractGitHubRepo(input)
		if result != "" {
			t.Errorf("extractGitHubRepo(%q) = %q, want empty string", input, result)
		}
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "v1.0.0"},
		{"v2.5.3", "v2.5.3"},
		{"1.0.0", "v1.0.0"},
		{"2.5.3", "v2.5.3"},
	}

	for _, tt := range tests {
		if result := normalizeVersion(tt.input); result != tt.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsSemverTag(t *testing.T) {
	tests := []struct {
		tag      string
		expected bool
	}{
		{"v1.0.0", true},
		{"v2.3.4-rc.1", true},
		{"operator/v0.9.0", false},
		{"1.0.0", false},
		{"v1.0", false},
		{"foo", false},
	}

	for _, tt := range tests {
		if got := isSemverTag(tt.tag); got != tt.expected {
			t.Errorf("isSemverTag(%q) = %v, want %v", tt.tag, got, tt.expected)
		}
	}
}

func TestLatestGoModuleVersionSkipsRetract(t *testing.T) {
	versions := []VersionInfo{
		{Version: "v1.9.0-retract", Time: time.Date(2025, 12, 3, 0, 0, 0, 0, time.UTC)},
		{Version: "v1.9.0", Time: time.Date(2025, 12, 2, 0, 0, 0, 0, time.UTC)},
		{Version: "v1.8.0", Time: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
	}

	if got := latestGoModuleVersion(versions); got != "v1.9.0" {
		t.Fatalf("latestGoModuleVersion() = %q, want %q", got, "v1.9.0")
	}
}

func TestLatestSemverTagPrefersSemanticRelease(t *testing.T) {
	releases := []Release{
		{Tag: "operator/v0.9.0"},
		{Tag: "v3.6.2"},
		{Tag: "v3.6.1"},
	}

	if tag := latestSemverTag(releases); tag != "v3.6.2" {
		t.Errorf("latestSemverTag = %q, want %q", tag, "v3.6.2")
	}
}

func TestLatestSemverTagNoneFound(t *testing.T) {
	releases := []Release{
		{Tag: "operator/v0.9.0"},
		{Tag: "foo"},
	}

	if tag := latestSemverTag(releases); tag != "" {
		t.Errorf("latestSemverTag = %q, want empty string", tag)
	}
}

func TestParseBasicOutput(t *testing.T) {
	output := "github.com/prometheus/common v0.55.0 v0.56.0 v0.57.0"
	versions := parseGoVersionsOutput(output)
	expected := []string{"v0.55.0", "v0.56.0", "v0.57.0"}

	if len(versions) != len(expected) {
		t.Fatalf("Expected %d versions, got %d", len(expected), len(versions))
	}

	for i, v := range expected {
		if versions[i] != v {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], v)
		}
	}
}

func TestParseWithManyVersions(t *testing.T) {
	output := "module/path v1.0.0 v1.1.0 v1.2.0 v1.3.0 v1.4.0 v2.0.0"
	versions := parseGoVersionsOutput(output)
	expected := []string{"v1.0.0", "v1.1.0", "v1.2.0", "v1.3.0", "v1.4.0", "v2.0.0"}

	if len(versions) != len(expected) {
		t.Fatalf("Expected %d versions, got %d", len(expected), len(versions))
	}

	for i, v := range expected {
		if versions[i] != v {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], v)
		}
	}
}

func TestParseWithPrerelease(t *testing.T) {
	output := "module/path v1.0.0-rc.1 v1.0.0 v1.1.0-beta.1"
	versions := parseGoVersionsOutput(output)
	expected := []string{"v1.0.0-rc.1", "v1.0.0", "v1.1.0-beta.1"}

	if len(versions) != len(expected) {
		t.Fatalf("Expected %d versions, got %d", len(expected), len(versions))
	}

	for i, v := range expected {
		if versions[i] != v {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], v)
		}
	}
}

func TestParseEmptyOutput(t *testing.T) {
	outputs := []string{"", "   "}
	for _, output := range outputs {
		versions := parseGoVersionsOutput(output)
		if len(versions) != 0 {
			t.Errorf("parseGoVersionsOutput(%q) should return empty slice", output)
		}
	}
}

func TestParseModuleOnly(t *testing.T) {
	output := "module/path"
	versions := parseGoVersionsOutput(output)
	if len(versions) != 0 {
		t.Error("parseGoVersionsOutput with only module name should return empty slice")
	}
}

func TestParseFiltersIncompatibleVersions(t *testing.T) {
	output := "github.com/prometheus/prometheus v0.305.0 v0.306.0+incompatible v0.307.0 v0.308.0+incompatible"
	versions := parseGoVersionsOutput(output)
	expected := []string{"v0.305.0", "v0.307.0"}

	if len(versions) != len(expected) {
		t.Fatalf("Expected %d versions, got %d", len(expected), len(versions))
	}

	for i, v := range expected {
		if versions[i] != v {
			t.Errorf("versions[%d] = %q, want %q", i, versions[i], v)
		}
	}
}

func TestParseBasicReleaseOutput(t *testing.T) {
	output := "v1.0.0\tLatest\tv1.0.0\t2024-01-15T10:30:00Z"
	releases := parseGitHubReleasesOutput(output)

	if len(releases) != 1 {
		t.Fatalf("Expected 1 release, got %d", len(releases))
	}

	if releases[0].Tag != "v1.0.0" {
		t.Errorf("Tag = %q, want %q", releases[0].Tag, "v1.0.0")
	}
	if releases[0].Title != "Latest - v1.0.0" {
		t.Errorf("Title = %q, want %q", releases[0].Title, "Latest - v1.0.0")
	}
	if releases[0].Published != "2024-01-15T10:30:00Z" {
		t.Errorf("Published = %q, want %q", releases[0].Published, "2024-01-15T10:30:00Z")
	}
}

func TestParseMultipleReleases(t *testing.T) {
	output := `v3.8.0	Latest	v3.8.0	2025-12-02T10:00:00Z
v3.7.3		v3.7.3	2025-10-30T15:30:00Z
v3.7.2		v3.7.2	2025-10-22T12:00:00Z`
	releases := parseGitHubReleasesOutput(output)

	if len(releases) != 3 {
		t.Fatalf("Expected 3 releases, got %d", len(releases))
	}

	if releases[0].Tag != "v3.8.0" {
		t.Errorf("releases[0].Tag = %q, want %q", releases[0].Tag, "v3.8.0")
	}
	if releases[1].Tag != "v3.7.3" {
		t.Errorf("releases[1].Tag = %q, want %q", releases[1].Tag, "v3.7.3")
	}
	if releases[2].Tag != "v3.7.2" {
		t.Errorf("releases[2].Tag = %q, want %q", releases[2].Tag, "v3.7.2")
	}
}

func TestParseGitHubReleaseWithPrerelease(t *testing.T) {
	output := "v3.8.0-rc.1\tPre-release\tv3.8.0-rc.1\t2025-11-24T08:00:00Z"
	releases := parseGitHubReleasesOutput(output)

	if len(releases) != 1 {
		t.Fatalf("Expected 1 release, got %d", len(releases))
	}

	if releases[0].Tag != "v3.8.0-rc.1" {
		t.Errorf("Tag = %q, want %q", releases[0].Tag, "v3.8.0-rc.1")
	}
	if releases[0].Title != "Pre-release - v3.8.0-rc.1" {
		t.Errorf("Title = %q, want %q", releases[0].Title, "Pre-release - v3.8.0-rc.1")
	}
}

func TestParseWithoutDate(t *testing.T) {
	output := "v1.0.0\tLatest\tv1.0.0"
	releases := parseGitHubReleasesOutput(output)

	if len(releases) != 1 {
		t.Fatalf("Expected 1 release, got %d", len(releases))
	}

	if releases[0].Published != "N/A" {
		t.Errorf("Published = %q, want %q", releases[0].Published, "N/A")
	}
}

func TestParseEmptyReleaseOutput(t *testing.T) {
	outputs := []string{"", "   "}
	for _, output := range outputs {
		releases := parseGitHubReleasesOutput(output)
		if len(releases) != 0 {
			t.Errorf("parseGitHubReleasesOutput(%q) should return empty slice", output)
		}
	}
}

func TestParseWithEmptyLines(t *testing.T) {
	output := `v3.8.0	Latest	v3.8.0	2025-12-02T10:00:00Z

v3.7.3		v3.7.3	2025-10-30T15:30:00Z`
	releases := parseGitHubReleasesOutput(output)

	if len(releases) != 2 {
		t.Fatalf("Expected 2 releases, got %d", len(releases))
	}
}

func TestAllOpenTelemetryMappings(t *testing.T) {
	mappings := map[string]string{
		"go.opentelemetry.io/otel":                "open-telemetry/opentelemetry-go",
		"go.opentelemetry.io/otel/sdk":            "open-telemetry/opentelemetry-go",
		"go.opentelemetry.io/contrib":             "open-telemetry/opentelemetry-go-contrib",
		"go.opentelemetry.io/contrib/otelconf":    "open-telemetry/opentelemetry-go-contrib",
		"go.opentelemetry.io/collector":           "open-telemetry/opentelemetry-collector",
		"go.opentelemetry.io/collector/client":    "open-telemetry/opentelemetry-collector",
		"go.opentelemetry.io/collector/component": "open-telemetry/opentelemetry-collector",
		"go.opentelemetry.io/build-tools":         "open-telemetry/opentelemetry-go-build-tools",
		"go.opentelemetry.io/auto":                "open-telemetry/opentelemetry-go-instrumentation",
		"go.opentelemetry.io/obi":                 "grafana/opentelemetry-ebpf-instrumentation",
		"go.opentelemetry.io/ebpf-profiler":       "grafana/opentelemetry-ebpf-profiler",
	}

	for module, expectedRepo := range mappings {
		actualRepo := extractGitHubRepo(module)
		if actualRepo != expectedRepo {
			t.Errorf("Failed for %q: expected %q, got %q", module, expectedRepo, actualRepo)
		}
	}
}

func TestGrafanaForkMappings(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"go.opentelemetry.io/obi", "grafana/opentelemetry-ebpf-instrumentation"},
		{"go.opentelemetry.io/ebpf-profiler", "grafana/opentelemetry-ebpf-profiler"},
	}

	for _, tt := range tests {
		result := extractGitHubRepo(tt.input)
		if result != tt.expected {
			t.Errorf("extractGitHubRepo(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestParseGitTagsOutput(t *testing.T) {
	output := `v0.0.202549	2024-12-01T10:00:00Z
v0.0.202545	2024-11-28T15:30:00Z
v0.0.202540	2024-11-25T09:15:00Z`

	releases := parseGitTagsOutput(output)

	if len(releases) != 3 {
		t.Fatalf("Expected 3 releases, got %d", len(releases))
	}

	if releases[0].Tag != "v0.0.202549" {
		t.Errorf("releases[0].Tag = %q, want %q", releases[0].Tag, "v0.0.202549")
	}
	if releases[0].Title != "v0.0.202549" {
		t.Errorf("releases[0].Title = %q, want %q", releases[0].Title, "v0.0.202549")
	}
	if releases[0].Published != "2024-12-01T10:00:00Z" {
		t.Errorf("releases[0].Published = %q, want %q", releases[0].Published, "2024-12-01T10:00:00Z")
	}

	if releases[1].Tag != "v0.0.202545" {
		t.Errorf("releases[1].Tag = %q, want %q", releases[1].Tag, "v0.0.202545")
	}
}

func TestParseGitTagsOutputWithoutDate(t *testing.T) {
	output := `v0.0.202549
v0.0.202545`

	releases := parseGitTagsOutput(output)

	if len(releases) != 2 {
		t.Fatalf("Expected 2 releases, got %d", len(releases))
	}

	if releases[0].Published != "N/A" {
		t.Errorf("releases[0].Published = %q, want %q", releases[0].Published, "N/A")
	}
}

func TestGetPrimaryLookupMethod(t *testing.T) {
	tests := []struct {
		modulePath string
		githubRepo string
		expected   LookupMethod
	}{
		{"go.opentelemetry.io/ebpf-profiler", "grafana/opentelemetry-ebpf-profiler", GitTag},
		{"go.opentelemetry.io/obi", "grafana/opentelemetry-ebpf-instrumentation", GitTag},
		{"github.com/prometheus/prometheus", "prometheus/prometheus", GitHubRelease},
		{"github.com/prometheus/common", "prometheus/common", GitHubRelease},
		{"github.com/grafana/loki/v3", "grafana/loki", GitHubRelease},
		{"go.opentelemetry.io/collector", "open-telemetry/opentelemetry-collector", GitHubRelease},
		{"github.com/stretchr/testify", "stretchr/testify", GitHubRelease},
		{"github.com/some/unknown-repo", "some/unknown-repo", GitHubRelease},
		{"golang.org/x/text", "", GoModule},
		{"gopkg.in/yaml.v3", "", GoModule},
	}

	for _, tt := range tests {
		if result := getPrimaryLookupMethod(tt.modulePath, tt.githubRepo); result != tt.expected {
			t.Errorf("getPrimaryLookupMethod(%q, %q) = %q, want %q", tt.modulePath, tt.githubRepo, result, tt.expected)
		}
	}
}
