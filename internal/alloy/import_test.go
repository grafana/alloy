package alloy_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/alloy"
	"github.com/grafana/alloy/internal/alloy/internal/testcomponents"
	"github.com/grafana/alloy/internal/alloy/logging"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/txtar"

	_ "github.com/grafana/alloy/internal/alloy/internal/testcomponents/module/string"
)

// use const to avoid lint error
const mainFile = "main.alloy"

// The tests are using the .txtar files stored in the testdata folder.
type testImportFile struct {
	description       string      // description at the top of the txtar file
	main              string      // root config that the controller should load
	module            string      // module imported by the root config
	nestedModule      string      // nested module that can be imported by the module
	reloadConfig      string      // root config that the controller should apply on reload
	otherNestedModule string      // another nested module
	update            *updateFile // update can be used to update the content of a file at runtime
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
	description string      // description at the top of the txtar file
	main        string      // root config that the controller should load
	module1     string      // module imported by the root config
	module2     string      // another module imported by the root config
	removed     string      // module will be removed in the dir on update
	added       string      // module which will be added in the dir on update
	update      *updateFile // update can be used to update the content of a file at runtime
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
	ctrl, f := setup(t, config)

	err := ctrl.LoadSource(f, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ctrl.Run(ctx)
	}()

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
		f, err = alloy.ParseSource(t.Name(), []byte(reloadConfig))
		require.NoError(t, err)
		require.NotNil(t, f)

		// Reload the controller with the new config.
		err = ctrl.LoadSource(f, nil)
		require.NoError(t, err)

		// Export should be -10 after update
		require.Eventually(t, func() bool {
			export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
			return export.LastAdded <= -10
		}, 3*time.Second, 10*time.Millisecond)
	}
}

func testConfigError(t *testing.T, config string, expectedError string) {
	defer verifyNoGoroutineLeaks(t)
	ctrl, f := setup(t, config)
	err := ctrl.LoadSource(f, nil)
	require.ErrorContains(t, err, expectedError)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		ctrl.Run(ctx)
	}()
}

func setup(t *testing.T, config string) (*alloy.Alloy, *alloy.Source) {
	s, err := logging.New(os.Stderr, logging.DefaultOptions)
	require.NoError(t, err)
	ctrl := alloy.New(alloy.Options{
		Logger:       s,
		DataPath:     t.TempDir(),
		MinStability: featuregate.StabilityPublicPreview,
		Reg:          nil,
		Services:     []service.Service{},
	})
	f, err := alloy.ParseSource(t.Name(), []byte(config))
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

	return files
}
