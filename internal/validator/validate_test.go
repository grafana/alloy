package validator

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	// Install Components
	_ "github.com/grafana/alloy/internal/component/all"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
)

// Set this flag to update snapshots e.g. `go test -v ./interal/validation/...` -fix-tests
var fixTestsFlag = flag.Bool("fix-tests", false, "update the test files with the current generated content")

const (
	txtarSuffix = ".txtar"
	diagsSuffix = ".diags"
)

func TestValidate(t *testing.T) {
	// Test with default config.
	testDirectory(t, "./testdata/default", featuregate.StabilityGenerallyAvailable, false)
}

func testDirectory(t *testing.T, dir string, minStability featuregate.Stability, enableCommunityComps bool) {
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, _ error) error {
		if d.IsDir() && path != dir {
			return filepath.SkipDir
		}

		if strings.HasSuffix(path, txtarSuffix) {
			archive, err := txtar.ParseFile(path)
			require.NoError(t, err)

			t.Run(strings.TrimSpace(string(archive.Comment)), func(t *testing.T) {
				sources := make(map[string][]byte, len(archive.Files))
				for _, f := range archive.Files {
					sources[f.Name] = f.Data
				}

				validateErr := Validate(Options{
					Sources:           sources,
					ComponentRegistry: component.NewDefaultRegistry(minStability, enableCommunityComps),
				})

				diagsFile := strings.TrimSuffix(path, txtarSuffix) + diagsSuffix

				if *fixTestsFlag {
					f, err := os.OpenFile(diagsFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
					require.NoError(t, err)
					if validateErr != nil {
						Report(f, validateErr, sources)
					}

					require.NoError(t, f.Close())
					t.Logf("updated diags file %s", diagsFile)
				}

				if !fileExists(diagsFile) {
					t.Fatalf("no expected diags for %s - missing test expectations. run with -fix-tests to create missing and update existing diag files", path)
				}

				snapshot, err := os.ReadFile(diagsFile)
				require.NoError(t, err)

				buffer := bytes.NewBuffer([]byte{})
				if validateErr != nil {
					Report(buffer, validateErr, sources)
				}

				assert.Equal(t, string(snapshot), buffer.String(), "Missmatching expected diagnostics")
			})
		}
		return nil
	}))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
