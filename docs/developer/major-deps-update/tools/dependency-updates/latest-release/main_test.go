package main

import (
	"testing"
)

func TestGitHubToGoBasic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v3.8.0", "v0.308.0"},
		{"v3.4.2", "v0.304.2"},
		{"v2.9.17", "v0.209.17"},
	}

	for _, tt := range tests {
		result := githubVersionToGoModule(tt.input)
		if result != tt.expected {
			t.Errorf("githubVersionToGoModule(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGitHubToGoWithPrerelease(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v3.8.0-rc.1", "v0.308.0-rc.1"},
		{"v3.7.0-rc.0", "v0.307.0-rc.0"},
		{"v2.5.0-beta.1", "v0.205.0-beta.1"},
	}

	for _, tt := range tests {
		result := githubVersionToGoModule(tt.input)
		if result != tt.expected {
			t.Errorf("githubVersionToGoModule(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGitHubToGoDoubleDigitMinor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v3.12.5", "v0.312.5"},
		{"v1.25.3", "v0.125.3"},
	}

	for _, tt := range tests {
		result := githubVersionToGoModule(tt.input)
		if result != tt.expected {
			t.Errorf("githubVersionToGoModule(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGitHubToGoInvalid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"invalid", "invalid"},
		{"v1.0", "v1.0"},
	}

	for _, tt := range tests {
		result := githubVersionToGoModule(tt.input)
		if result != tt.expected {
			t.Errorf("githubVersionToGoModule(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGoToGitHubBasic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v0.308.0", "v3.8.0"},
		{"v0.304.2", "v3.4.2"},
		{"v0.209.17", "v2.9.17"},
	}

	for _, tt := range tests {
		result := goModuleVersionToGitHub(tt.input)
		if result != tt.expected {
			t.Errorf("goModuleVersionToGitHub(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGoToGitHubWithPrerelease(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v0.308.0-rc.1", "v3.8.0-rc.1"},
		{"v0.307.0-rc.0", "v3.7.0-rc.0"},
		{"v0.205.0-beta.1", "v2.5.0-beta.1"},
	}

	for _, tt := range tests {
		result := goModuleVersionToGitHub(tt.input)
		if result != tt.expected {
			t.Errorf("goModuleVersionToGitHub(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGoToGitHubDoubleDigitMinor(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v0.312.5", "v3.12.5"},
		{"v0.125.3", "v1.25.3"},
	}

	for _, tt := range tests {
		result := goModuleVersionToGitHub(tt.input)
		if result != tt.expected {
			t.Errorf("goModuleVersionToGitHub(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGoToGitHubInvalid(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"invalid", "invalid"},
		{"v1.0.0", "v1.0.0"},
	}

	for _, tt := range tests {
		result := goModuleVersionToGitHub(tt.input)
		if result != tt.expected {
			t.Errorf("goModuleVersionToGitHub(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestBidirectionalConversion(t *testing.T) {
	githubVersions := []string{"v3.8.0", "v2.12.5", "v1.0.3", "v4.15.0-rc.1"}
	for _, ghVer := range githubVersions {
		goVer := githubVersionToGoModule(ghVer)
		result := goModuleVersionToGitHub(goVer)
		if result != ghVer {
			t.Errorf("Bidirectional conversion failed for %q: got %q", ghVer, result)
		}
	}

	goVersions := []string{"v0.308.0", "v0.212.5", "v0.100.3", "v0.415.0-rc.1"}
	for _, goVer := range goVersions {
		ghVer := goModuleVersionToGitHub(goVer)
		result := githubVersionToGoModule(ghVer)
		if result != goVer {
			t.Errorf("Bidirectional conversion failed for %q: got %q", goVer, result)
		}
	}
}

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

func TestPrometheusDetected(t *testing.T) {
	if !usesSpecialVersioning("prometheus/prometheus") {
		t.Error("prometheus/prometheus should use special versioning")
	}
}

func TestOtherReposNotDetected(t *testing.T) {
	repos := []string{
		"prometheus/common",
		"grafana/loki",
		"stretchr/testify",
		"open-telemetry/opentelemetry-go",
	}

	for _, repo := range repos {
		if usesSpecialVersioning(repo) {
			t.Errorf("%q should not use special versioning", repo)
		}
	}
}

func TestNormalizeWithVPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"v1.0.0", "v1.0.0"},
		{"v2.5.3", "v2.5.3"},
	}

	for _, tt := range tests {
		result := normalizeVersion(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeWithoutVPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1.0.0", "v1.0.0"},
		{"2.5.3", "v2.5.3"},
	}

	for _, tt := range tests {
		result := normalizeVersion(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeVersion(%q) = %q, want %q", tt.input, result, tt.expected)
		}
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

func TestPrometheusPrometheusOutput(t *testing.T) {
	goOutput := "github.com/prometheus/prometheus v0.305.0 v0.306.0 v0.307.0 v0.308.0"
	versions := parseGoVersionsOutput(goOutput)

	if versions[len(versions)-1] != "v0.308.0" {
		t.Errorf("Latest version = %q, want %q", versions[len(versions)-1], "v0.308.0")
	}

	if goModuleVersionToGitHub("v0.308.0") != "v3.8.0" {
		t.Error("Conversion v0.308.0 -> v3.8.0 failed")
	}

	ghOutput := `v3.8.0	Latest	v3.8.0	2025-12-02T10:00:00Z
v3.7.3		v3.7.3	2025-10-30T15:30:00Z`
	releases := parseGitHubReleasesOutput(ghOutput)

	if releases[0].Tag != "v3.8.0" {
		t.Errorf("Latest GitHub tag = %q, want %q", releases[0].Tag, "v3.8.0")
	}

	if githubVersionToGoModule("v3.8.0") != "v0.308.0" {
		t.Error("Conversion v3.8.0 -> v0.308.0 failed")
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
		"go.opentelemetry.io/obi":                 "open-telemetry/opentelemetry-ebpf-instrumentation",
		"go.opentelemetry.io/ebpf-profiler":       "open-telemetry/opentelemetry-ebpf-profiler",
	}

	for module, expectedRepo := range mappings {
		actualRepo := extractGitHubRepo(module)
		if actualRepo != expectedRepo {
			t.Errorf("Failed for %q: expected %q, got %q", module, expectedRepo, actualRepo)
		}
	}
}
