package runtime_test

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	_ "github.com/grafana/alloy/internal/runtime/internal/testcomponents/module/string"
	"github.com/grafana/alloy/internal/runtime/internal/testservices"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/util"
)

// use const to avoid lint error
const mainFile = "main.alloy"

// The tests are using the .txtar files stored in the testdata folder.
type testImportFile struct {
	description            string      // description at the top of the txtar file
	main                   string      // root config that the controller should load
	module                 string      // module imported by the root config
	nestedModule           string      // nested module that can be imported by the module
	reloadConfig           string      // root config that the controller should apply on reload
	otherNestedModule      string      // another nested module
	nestedPathModule       string      // a module in a subdirectory
	deeplyNestedPathModule string      // a module in a sub-subdirectory
	update                 *updateFile // update can be used to update the content of a file at runtime
}

type updateFile struct {
	name         string // name of the file which should be updated
	updateConfig string // new module config which should be used
}

func buildTestImportFile(t *testing.T, filename string) testImportFile {
	archive, err := txtar.ParseFile(filename)
	require.NoError(t, err)
	var tc testImportFile
	tc.description = string(archive.Comment)
	for _, alloyConfig := range archive.Files {
		switch alloyConfig.Name {
		case mainFile:
			tc.main = string(alloyConfig.Data)
		case "module.alloy":
			tc.module = string(alloyConfig.Data)
		case "nested_module.alloy":
			tc.nestedModule = string(alloyConfig.Data)
		case "update/module.alloy":
			require.Nil(t, tc.update)
			tc.update = &updateFile{
				name:         "module.alloy",
				updateConfig: string(alloyConfig.Data),
			}
		case "update/nested_module.alloy":
			require.Nil(t, tc.update)
			tc.update = &updateFile{
				name:         "nested_module.alloy",
				updateConfig: string(alloyConfig.Data),
			}
		case "reload_config.alloy":
			tc.reloadConfig = string(alloyConfig.Data)
		case "other_nested_module.alloy":
			tc.otherNestedModule = string(alloyConfig.Data)
		case "nested_test/module.alloy":
			tc.nestedPathModule = string(alloyConfig.Data)
		case "nested_test/utils/module.alloy":
			tc.deeplyNestedPathModule = string(alloyConfig.Data)
		}
	}
	return tc
}

func TestImportFile(t *testing.T) {
	directory := "./testdata/import_file"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestImportFile(t, filepath.Join(directory, file.Name()))
		t.Run(tc.description, func(t *testing.T) {
			defer os.Remove("module.alloy")
			require.NoError(t, os.WriteFile("module.alloy", []byte(tc.module), 0664))
			if tc.nestedModule != "" {
				defer os.Remove("nested_module.alloy")
				require.NoError(t, os.WriteFile("nested_module.alloy", []byte(tc.nestedModule), 0664))
			}
			if tc.otherNestedModule != "" {
				defer os.Remove("other_nested_module.alloy")
				require.NoError(t, os.WriteFile("other_nested_module.alloy", []byte(tc.otherNestedModule), 0664))
			}

			if tc.nestedPathModule != "" || tc.deeplyNestedPathModule != "" {
				require.NoError(t, os.Mkdir("nested_test", 0700))
				defer os.RemoveAll("nested_test")
				if tc.nestedPathModule != "" {
					require.NoError(t, os.WriteFile("nested_test/module.alloy", []byte(tc.nestedPathModule), 0664))
				}
				if tc.deeplyNestedPathModule != "" {
					require.NoError(t, os.Mkdir("nested_test/utils", 0700))
					require.NoError(t, os.WriteFile("nested_test/utils/module.alloy", []byte(tc.deeplyNestedPathModule), 0664))
				}
			}

			if tc.update != nil {
				testConfig(t, tc.main, tc.reloadConfig, func() {
					require.NoError(t, os.WriteFile(tc.update.name, []byte(tc.update.updateConfig), 0664))
				})
			} else {
				testConfig(t, tc.main, tc.reloadConfig, nil)
			}
		})
	}
}

func TestImportString(t *testing.T) {
	directory := "./testdata/import_string"
	for _, file := range getTestFiles(directory, t) {
		archive, err := txtar.ParseFile(filepath.Join(directory, file.Name()))
		require.NoError(t, err)
		t.Run(archive.Files[0].Name, func(t *testing.T) {
			testConfig(t, string(archive.Files[0].Data), "", nil)
		})
	}
}

func TestImportGit(t *testing.T) {
	// Extract repo.git.tar so tests can make use of it.
	// Make repo.git.tar with:
	//   tar -C repo.git -cvf repo.git.tar .
	// NOTE: when modifying the files in the repo, make sure to commit the files else
	// the changes will not be taken into account.
	require.NoError(t, util.Untar("./testdata/repo.git.tar", "./testdata/repo.git"))
	require.NoError(t, util.Untar("./testdata/repo2.git.tar", "./testdata/repo2.git"))
	t.Cleanup(func() {
		_ = os.RemoveAll("./testdata/repo.git")
		_ = os.RemoveAll("./testdata/repo2.git")
	})

	directory := "./testdata/import_git"
	for _, file := range getTestFiles(directory, t) {
		archive, err := txtar.ParseFile(filepath.Join(directory, file.Name()))
		require.NoError(t, err)
		t.Run(archive.Files[0].Name, func(t *testing.T) {
			testConfig(t, string(archive.Files[0].Data), "", nil)
		})
	}
}

func TestImportHTTP(t *testing.T) {
	directory := "./testdata/import_http"
	for _, file := range getTestFiles(directory, t) {
		archive, err := txtar.ParseFile(filepath.Join(directory, file.Name()))
		require.NoError(t, err)
		t.Run(archive.Files[0].Name, func(t *testing.T) {
			testConfig(t, string(archive.Files[0].Data), "", nil)
		})
	}
}

type testImportFileFolder struct {
	description  string      // description at the top of the txtar file
	main         string      // root config that the controller should load
	module1      string      // module imported by the root config
	module2      string      // another module imported by the root config
	utilsModule2 string      // another module in a nested subdirectory
	removed      string      // module will be removed in the dir on update
	added        string      // module which will be added in the dir on update
	update       *updateFile // update can be used to update the content of a file at runtime
}

func buildTestImportFileFolder(t *testing.T, filename string) testImportFileFolder {
	archive, err := txtar.ParseFile(filename)
	require.NoError(t, err)
	var tc testImportFileFolder
	tc.description = string(archive.Comment)
	for _, alloyConfig := range archive.Files {
		switch alloyConfig.Name {
		case mainFile:
			tc.main = string(alloyConfig.Data)
		case "module1.alloy":
			tc.module1 = string(alloyConfig.Data)
		case "module2.alloy":
			tc.module2 = string(alloyConfig.Data)
		case "utils/module2.alloy":
			tc.utilsModule2 = string(alloyConfig.Data)
		case "added.alloy":
			tc.added = string(alloyConfig.Data)
		case "removed.alloy":
			tc.removed = string(alloyConfig.Data)
		case "update/module1.alloy":
			require.Nil(t, tc.update)
			tc.update = &updateFile{
				name:         "module1.alloy",
				updateConfig: string(alloyConfig.Data),
			}
		case "update/module2.alloy":
			require.Nil(t, tc.update)
			tc.update = &updateFile{
				name:         "module2.alloy",
				updateConfig: string(alloyConfig.Data),
			}
		case "utils/update_module2.alloy":
			require.Nil(t, tc.update)
			tc.update = &updateFile{
				name:         "utils/module2.alloy",
				updateConfig: string(alloyConfig.Data),
			}
		}
	}
	return tc
}

func TestImportFileFolder(t *testing.T) {
	directory := "./testdata/import_file_folder"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestImportFileFolder(t, filepath.Join(directory, file.Name()))
		t.Run(tc.description, func(t *testing.T) {
			dir := "tmpTest"
			require.NoError(t, os.Mkdir(dir, 0700))
			defer os.RemoveAll(dir)

			if tc.module1 != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "module1.alloy"), []byte(tc.module1), 0700))
			}

			if tc.module2 != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "module2.alloy"), []byte(tc.module2), 0700))
			}

			if tc.removed != "" {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "removed.alloy"), []byte(tc.removed), 0700))
			}

			if tc.utilsModule2 != "" {
				nestedDir := filepath.Join(dir, "utils")
				require.NoError(t, os.Mkdir(nestedDir, 0700))
				require.NoError(t, os.WriteFile(filepath.Join(nestedDir, "module2.alloy"), []byte(tc.utilsModule2), 0700))
			}

			// TODO: ideally we would like to check the health of the node but that's not yet possible for import nodes.
			// We should expect that adding or removing files in the dir is gracefully handled and the node should be
			// healthy once it polls the content of the dir again.
			testConfig(t, tc.main, "", func() {
				if tc.removed != "" {
					os.Remove(filepath.Join(dir, "removed.alloy"))
				}

				if tc.added != "" {
					require.NoError(t, os.WriteFile(filepath.Join(dir, "added.alloy"), []byte(tc.added), 0700))
				}
				if tc.update != nil {
					require.NoError(t, os.WriteFile(filepath.Join(dir, tc.update.name), []byte(tc.update.updateConfig), 0700))
				}
			})
		})
	}
}

type testImportError struct {
	description   string
	main          string
	expectedError string
}

func buildTestImportError(t *testing.T, filename string) testImportError {
	archive, err := txtar.ParseFile(filename)
	require.NoError(t, err)
	var tc testImportError
	tc.description = string(archive.Comment)
	for _, alloyConfig := range archive.Files {
		switch alloyConfig.Name {
		case mainFile:
			tc.main = string(alloyConfig.Data)
		case "error":
			tc.expectedError = string(alloyConfig.Data)
		}
	}
	return tc
}

func TestImportError(t *testing.T) {
	directory := "./testdata/import_error"
	for _, file := range getTestFiles(directory, t) {
		tc := buildTestImportError(t, filepath.Join(directory, file.Name()))
		t.Run(tc.description, func(t *testing.T) {
			testConfigError(t, tc.main, strings.TrimRight(tc.expectedError, "\n"))
		})
	}
}

func testConfig(t *testing.T, config string, reloadConfig string, update func()) {
	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, config, nil, featuregate.StabilityPublicPreview)

	err := ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Go(func() {
		ctrl.Run(ctx)
	})

	require.Eventually(t, func() bool {
		return ctrl.LoadComplete()
	}, 3*time.Second, 10*time.Millisecond)

	// Check for initial condition
	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
		return export.LastAdded >= 10
	}, 3*time.Second, 10*time.Millisecond)

	if update != nil {
		update()

		// Export should be -10 after update
		require.Eventually(t, func() bool {
			export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
			return export.LastAdded <= -10
		}, 3*time.Second, 10*time.Millisecond)
	}

	if reloadConfig != "" {
		f, err = alloy_runtime.ParseSource(t.Name(), []byte(reloadConfig))
		require.NoError(t, err)
		require.NotNil(t, f)

		// Reload the controller with the new config.
		err = ctrl.LoadSource(f, nil, "")
		require.NoError(t, err)

		require.Eventually(t, func() bool {
			return ctrl.LoadComplete()
		}, 3*time.Second, 10*time.Millisecond)

		// Export should be -10 after update
		require.Eventually(t, func() bool {
			export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
			return export.LastAdded <= -10
		}, 3*time.Second, 10*time.Millisecond)
	}
}

func testConfigError(t *testing.T, config string, expectedError string) {
	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, config, nil, featuregate.StabilityPublicPreview)
	err := ctrl.LoadSource(f, nil, "")
	require.ErrorContains(t, err, expectedError)
	ctx, cancel := context.WithCancel(t.Context())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Go(func() {
		ctrl.Run(ctx)
	})
}

func setup(t *testing.T, config string, reg prometheus.Registerer, stability featuregate.Stability) (*alloy_runtime.Runtime, *alloy_runtime.Source) {
	s, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)
	ctrl := alloy_runtime.New(alloy_runtime.Options{
		Logger:       s,
		DataPath:     t.TempDir(),
		MinStability: stability,
		Reg:          reg,
		Services: []service.Service{
			&testservices.Fake{},
		},
	})
	f, err := alloy_runtime.ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)
	return ctrl, f
}

func getTestFiles(directory string, t *testing.T) []fs.FileInfo {
	dir, err := os.Open(directory)
	require.NoError(t, err)
	defer dir.Close()

	files, err := dir.Readdir(-1)
	require.NoError(t, err)

	// Don't use files which start with a dot (".").
	// This is to prevent the test suite from using files such as ".DS_Store",
	// which Visual Studio Code may add.
	return filterFiles(files, ".")
}

// Only take into account files which don't have a certain prefix.
func filterFiles(files []fs.FileInfo, denylistedPrefix string) []fs.FileInfo {
	res := make([]fs.FileInfo, 0, len(files))
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), denylistedPrefix) {
			res = append(res, file)
		}
	}
	return res
}
