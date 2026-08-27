package syncreplaces

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractSharedReplacesFromBuilderConfig(t *testing.T) {
	const builderConfig = `replaces:
  # Local modules.
  - github.com/grafana/alloy => ../
  - github.com/grafana/alloy/syntax => ../syntax

  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  # Replace yaml.v2 with fork
  - gopkg.in/yaml.v2 => example.com/forks/go-yaml v0.0.0-20211119180816-77389c3526dc
  # Keep this dependency pinned
  # Until upstream publishes a compatible release
  - example.com/module => example.com/fork v1.2.3
  # <END_SHARED_REPLACE_DIRECTIVES>
`

	replaces, err := extractSharedReplaces([]byte(builderConfig))
	require.NoError(t, err)

	require.Len(t, replaces, 2)
	require.Equal(t, "gopkg.in/yaml.v2 => example.com/forks/go-yaml v0.0.0-20211119180816-77389c3526dc", replaces[0].Value)
	require.Equal(t, []string{"Replace yaml.v2 with fork"}, replaces[0].Comments)
	require.Equal(t, []string{"Keep this dependency pinned", "Until upstream publishes a compatible release"}, replaces[1].Comments)
}

func TestExtractSharedReplacesRequiresMarkers(t *testing.T) {
	_, err := extractSharedReplaces([]byte("replaces:\n  - example.com/module => example.com/fork v1.2.3\n"))
	require.ErrorContains(t, err, "missing shared replace start marker")
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
		{
			name: "invalid replace syntax",
			builderConfig: `replaces:
  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  # Replace module
  - example.com/module = > example.com/fork v1.2.3
  # <END_SHARED_REPLACE_DIRECTIVES>
`,
			wantErr: "invalid shared replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractSharedReplaces([]byte(tt.builderConfig))
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSyncBuilderConfigReplacesToGoModIdempotence(t *testing.T) {
	dir := t.TempDir()
	builderConfigPath := filepath.Join(dir, "builder-config.yaml")
	goModPath := filepath.Join(dir, "go.mod")

	const builderConfig = `replaces:
  # Local modules.
  - github.com/grafana/alloy => ../
  # <BEGIN_SHARED_REPLACE_DIRECTIVES>
  # Replace yaml.v2 with fork
  - gopkg.in/yaml.v2 => example.com/forks/go-yaml v0.0.0-20211119180816-77389c3526dc
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

	require.NoError(t, os.WriteFile(builderConfigPath, []byte(builderConfig), 0o644))
	require.NoError(t, os.WriteFile(goModPath, []byte(goMod), 0o644))

	require.NoError(t, syncBuilderConfigReplacesToGoMod(builderConfigPath, goModPath))

	data, err := os.ReadFile(goModPath)
	require.NoError(t, err)
	got := string(data)
	require.NotContains(t, got, "old.example/module", "stale replace was not removed")
	require.NotContains(t, got, "// stale", "stale replace comment was not removed")
	require.Contains(t, got, "replace github.com/grafana/alloy/syntax => ./syntax", "root-local path replace should be preserved")
	require.Contains(t, got, syncedReplaceBanner, "synced replace banner was not added")
	require.Contains(t, got, strings.TrimSpace(syncedReplaceBanner)+"\n\n// Replace yaml.v2 with fork", "synced replace banner should be separated from the first replace comment")
	require.Contains(t, got, "replace gopkg.in/yaml.v2 => example.com/forks/go-yaml", "shared replace was not propagated")
	require.Contains(t, got, "// Replace yaml.v2 with fork"+syncedCommentSuffix, "shared replace comment was not propagated")
	require.Contains(t, got, "// Keep this dependency pinned"+syncedCommentSuffix+"\n// Until upstream publishes a compatible release"+syncedCommentSuffix+"\nreplace example.com/module => example.com/fork v1.2.3", "multi-line shared replace comment was not propagated")

	// Run the full sync again to verify the generated block is replaced instead
	// of duplicating comments or replace directives.
	require.NoError(t, syncBuilderConfigReplacesToGoMod(builderConfigPath, goModPath))
	data, err = os.ReadFile(goModPath)
	require.NoError(t, err)
	got = string(data)
	require.Equal(t, 1, strings.Count(got, syncedReplaceBanner), "synced replace banner should not be duplicated after second sync")
	require.Contains(t, got, strings.TrimSpace(syncedReplaceBanner)+"\n\n// Replace yaml.v2 with fork", "synced replace banner should stay separated from the first replace comment after second sync")
	require.Equal(t, 3, strings.Count(got, syncedCommentSuffix), "synced comment suffix should not be duplicated after second sync")
}
