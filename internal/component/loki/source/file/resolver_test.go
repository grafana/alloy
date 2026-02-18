package file

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/discovery"
)

func TestResolver(t *testing.T) {
	type testCase struct {
		name     string
		resolver resolver
		targets  []discovery.Target
		expected []resolvedTarget
	}

	dir, err := os.Getwd()
	require.NoError(t, err)

	tests := []testCase{
		{
			name:     "static resolver",
			resolver: newStaticResolver(),
			targets: []discovery.Target{
				discovery.NewTargetFromLabelSet(model.LabelSet{
					"__path__":     "some path",
					"__internal__": "internal",
					"label":        "label",
				}),
			},
			expected: []resolvedTarget{
				{
					Path: "some path",
					Labels: model.LabelSet{
						"label": "label",
					},
				},
			},
		},
		{
			name:     "glob resolver",
			resolver: newGlobResolver(log.NewNopLogger()),
			targets: []discovery.Target{
				discovery.NewTargetFromLabelSet(model.LabelSet{
					"__path__":     "./testdata/*.log",
					"__internal__": "internal",
					"label":        "label",
				}),
			},
			expected: []resolvedTarget{
				{
					Path: filepath.Join(dir, "/testdata/onelinelog.log"),
					Labels: model.LabelSet{
						"label": "label",
					},
				},
				{
					Path: filepath.Join(dir, "/testdata/short-access.log"),
					Labels: model.LabelSet{
						"label": "label",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := 0
			for target := range tt.resolver.Resolve(tt.targets) {
				require.Equal(t, tt.expected[i], target)
				i += 1
			}
		})
	}
}

// TestGlobResolverMultiplePatterns verifies that the {a,b,c} pattern syntax works
// for matching multiple file extensions, multiple directories, and excluding multiple
// file types in a single glob pattern. This mirrors the documentation example.
func TestGlobResolverMultiplePatterns(t *testing.T) {
	// Create temp directory with subdirectories matching the documentation example
	dir := t.TempDir()

	// Create directory structure: {nginx,apache,caddy}/*.{log,txt,json} excluding *.{gz,zip,bak,old}
	for _, subdir := range []string{"nginx", "apache", "caddy", "other"} {
		subdirPath := filepath.Join(dir, subdir)
		err := os.MkdirAll(subdirPath, 0755)
		require.NoError(t, err)

		// Create files with various extensions in each directory
		testFiles := []string{
			"access.log",
			"error.txt",
			"config.json",
			"debug.yaml",      // Should not match (wrong extension)
			"access.log.gz",   // Should be excluded
			"error.txt.zip",   // Should be excluded
			"config.json.bak", // Should be excluded
			"old.log.old",     // Should be excluded
		}
		for _, f := range testFiles {
			err := os.WriteFile(filepath.Join(subdirPath, f), []byte("test"), 0644)
			require.NoError(t, err)
		}
	}

	resolver := newGlobResolver(log.NewNopLogger())

	// Use pattern matching multiple directories and extensions with exclusions
	// This mirrors: /var/log/{nginx,apache,caddy}/*.{log,txt,json} excluding *.{gz,zip,bak,old}
	targets := []discovery.Target{
		discovery.NewTargetFromLabelSet(model.LabelSet{
			"__path__":         model.LabelValue(filepath.Join(dir, "{nginx,apache,caddy}", "*.{log,txt,json}")),
			"__path_exclude__": model.LabelValue(filepath.Join(dir, "{nginx,apache,caddy}", "*.{gz,zip,bak,old}")),
			"label":            "test",
		}),
	}

	var results []resolvedTarget
	for target := range resolver.Resolve(targets) {
		results = append(results, target)
	}

	// Expected files: 3 directories × 3 extensions
	expectedFiles := []string{
		filepath.Join("nginx", "access.log"),
		filepath.Join("nginx", "error.txt"),
		filepath.Join("nginx", "config.json"),
		filepath.Join("apache", "access.log"),
		filepath.Join("apache", "error.txt"),
		filepath.Join("apache", "config.json"),
		filepath.Join("caddy", "access.log"),
		filepath.Join("caddy", "error.txt"),
		filepath.Join("caddy", "config.json"),
	}

	require.Len(t, results, len(expectedFiles),
		"Expected %d files: %v", len(expectedFiles), expectedFiles)

	// Collect all matched paths for verification
	var paths []string
	for _, r := range results {
		// Get relative path from dir for easier checking
		relPath, _ := filepath.Rel(dir, r.Path)
		paths = append(paths, relPath)
	}
	sort.Strings(paths)

	// Verify all expected files are matched
	for _, expected := range expectedFiles {
		require.Contains(t, paths, expected, "%s should be matched", expected)
	}

	// Verify files from "other" directory are NOT matched (wrong directory)
	require.NotContains(t, paths, filepath.Join("other", "access.log"),
		"other/access.log should not be matched (wrong directory)")

	// Verify excluded extensions are NOT matched
	for _, subdir := range []string{"nginx", "apache", "caddy"} {
		require.NotContains(t, paths, filepath.Join(subdir, "access.log.gz"),
			".gz files should be excluded")
		require.NotContains(t, paths, filepath.Join(subdir, "error.txt.zip"),
			".zip files should be excluded")
		require.NotContains(t, paths, filepath.Join(subdir, "config.json.bak"),
			".bak files should be excluded")
		require.NotContains(t, paths, filepath.Join(subdir, "old.log.old"),
			".old files should be excluded")
	}

	// Verify wrong extensions are NOT matched
	for _, subdir := range []string{"nginx", "apache", "caddy"} {
		require.NotContains(t, paths, filepath.Join(subdir, "debug.yaml"),
			".yaml files should not be matched")
	}
}
