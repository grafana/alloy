package syncreplaces

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractSharedReplacesFromBuilderConfig(t *testing.T) {
	const builderConfig = `replaces:
  # Local modules.
  - github.com/grafana/alloy => ../
  - github.com/grafana/alloy/syntax => ../syntax

  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  # Replace yaml.v2 with fork
  - gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20211119180816-77389c3526dc
  # Keep this dependency pinned
  # Until upstream publishes a compatible release
  - example.com/module => example.com/fork v1.2.3
  # <END_SHARED_REPLACE_DIRECTIVES>
`

	replaces, err := extractSharedReplaces([]byte(builderConfig))
	if err != nil {
		t.Fatalf("extractSharedReplaces returned error: %v", err)
	}

	if got, want := len(replaces), 2; got != want {
		t.Fatalf("expected %d replaces, got %d", want, got)
	}
	if got, want := replaces[0].Value, "gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20211119180816-77389c3526dc"; got != want {
		t.Fatalf("first replace value = %q, want %q", got, want)
	}
	if got, want := replaces[0].Comments, []string{"Replace yaml.v2 with fork"}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("first replace comments = %#v, want %#v", got, want)
	}
	if got, want := replaces[1].Comments, []string{"Keep this dependency pinned", "Until upstream publishes a compatible release"}; strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("second replace comments = %#v, want %#v", got, want)
	}
}

func TestExtractSharedReplacesRequiresMarkers(t *testing.T) {
	_, err := extractSharedReplaces([]byte("replaces:\n  - example.com/module => example.com/fork v1.2.3\n"))
	if err == nil {
		t.Fatal("expected missing marker error")
	}
	if !strings.Contains(err.Error(), "missing shared replace markers") {
		t.Fatalf("expected missing marker error, got %v", err)
	}
}

func TestExtractSharedReplacesRejectsMalformedSharedBlock(t *testing.T) {
	tests := []struct {
		name          string
		builderConfig string
		wantErr       string
	}{
		{
			name: "missing end",
			builderConfig: `replaces:
  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  # Replace module
  - example.com/module => example.com/fork v1.2.3
`,
			wantErr: "missing shared replace end marker",
		},
		{
			name: "entry without comment",
			builderConfig: `replaces:
  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  - example.com/module => example.com/fork v1.2.3
  # <END_SHARED_REPLACE_DIRECTIVES>
`,
			wantErr: "must have a comment",
		},
		{
			name: "unexpected line",
			builderConfig: `replaces:
  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  unexpected
  # <END_SHARED_REPLACE_DIRECTIVES>
`,
			wantErr: "must be a comment or replace entry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractSharedReplaces([]byte(tt.builderConfig))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestSyncBuilderConfigReplacesToGoMod(t *testing.T) {
	dir := t.TempDir()
	builderConfigPath := filepath.Join(dir, "builder-config.yaml")
	goModPath := filepath.Join(dir, "go.mod")

	const builderConfig = `replaces:
  # Local modules.
  - github.com/grafana/alloy => ../
  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  # Replace yaml.v2 with fork
  - gopkg.in/yaml.v2 => github.com/rfratto/go-yaml v0.0.0-20211119180816-77389c3526dc
  # Keep this dependency pinned
  # Until upstream publishes a compatible release
  - example.com/module => example.com/fork v1.2.3
  # <END_SHARED_REPLACE_DIRECTIVES>
`
	const goMod = `module example.com/project

go 1.26.2

// This local replace is intentionally root-only.
replace github.com/grafana/alloy/syntax => ./syntax

// stale
replace old.example/module => old.example/fork v0.0.1
`

	if err := os.WriteFile(builderConfigPath, []byte(builderConfig), 0o644); err != nil {
		t.Fatalf("write builder config: %v", err)
	}
	if err := os.WriteFile(goModPath, []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if err := syncBuilderConfigReplacesToGoMod(builderConfigPath, goModPath); err != nil {
		t.Fatalf("syncBuilderConfigReplacesToGoMod returned error: %v", err)
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "old.example/module") {
		t.Fatalf("stale replace was not removed:\n%s", got)
	}
	if !strings.Contains(got, "replace github.com/grafana/alloy/syntax => ./syntax") {
		t.Fatalf("root-local path replace should be preserved:\n%s", got)
	}
	if !strings.Contains(got, "replace gopkg.in/yaml.v2 => github.com/rfratto/go-yaml") {
		t.Fatalf("shared replace was not propagated:\n%s", got)
	}
	if !strings.Contains(got, "// Replace yaml.v2 with fork (synced from collector/builder-config.yaml)") {
		t.Fatalf("shared replace comment was not propagated:\n%s", got)
	}
	if !strings.Contains(got, "// Keep this dependency pinned (synced from collector/builder-config.yaml)\n// Until upstream publishes a compatible release (synced from collector/builder-config.yaml)\nreplace example.com/module => example.com/fork v1.2.3") {
		t.Fatalf("multi-line shared replace comment was not propagated:\n%s", got)
	}

	if err := syncBuilderConfigReplacesToGoMod(builderConfigPath, goModPath); err != nil {
		t.Fatalf("second syncBuilderConfigReplacesToGoMod returned error: %v", err)
	}
	data, err = os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod after second sync: %v", err)
	}
	got = string(data)
	if strings.Count(got, "(synced from collector/builder-config.yaml)") != 3 {
		t.Fatalf("synced comment suffix should not be duplicated after second sync:\n%s", got)
	}
}
