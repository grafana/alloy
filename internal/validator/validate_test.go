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
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/service/otel"
	"github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/grafana/alloy/internal/service/ui"

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
	testDirectory(t, "./testdata/ga", featuregate.StabilityGenerallyAvailable, false)
	testDirectory(t, "./testdata/default", featuregate.StabilityExperimental, false)
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
					ServiceDefinitions: getServiceDefinitions(
						&cluster.Service{},
						&http.Service{},
						&labelstore.Service{},
						&livedebugging.Service{},
						&otel.Service{},
						&remotecfg.Service{},
						&ui.Service{},
					),
					MinStability: minStability,
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

				expected := string(normalizeLineEndings(snapshot))
				actual := string(normalizeLineEndings(buffer.Bytes()))
				assert.Equal(t, expected, actual, "Missmatching expected diagnostics")
			})
		}
		return nil
	}))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// normalizeLineEndings will replace '\r\n' with '\n'.
func normalizeLineEndings(data []byte) []byte {
	normalized := bytes.ReplaceAll(data, []byte{'\r', '\n'}, []byte{'\n'})
	return normalized
}

func getServiceDefinitions(services ...service.Service) []service.Definition {
	def := make([]service.Definition, 0, len(services))
	for _, s := range services {
		def = append(def, s.Definition())
	}
	return def
}
