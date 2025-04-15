package validator

import (
	"bytes"
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

			t.Run(strings.TrimSuffix(string(archive.Comment), "\n"), func(t *testing.T) {
				sources := make(map[string][]byte, len(archive.Files))
				for _, f := range archive.Files {
					sources[f.Name] = f.Data
				}

				validateErr := Validate(sources, Options{
					ComponentRegistry: component.NewDefaultRegistry(minStability, enableCommunityComps),
				})

				diagsFile := strings.TrimSuffix(path, txtarSuffix) + diagsSuffix
				if !fileExists(diagsFile) {
					f, createErr := os.Create(diagsFile)
					require.NoError(t, createErr)
					if validateErr != nil {
						Report(f, validateErr, sources)
					}
					require.NoError(t, f.Close())
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
