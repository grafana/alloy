package pipelinetest

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tempDirVar is the placeholder for the working directory of a test. It
// works in the config, in file paths and in commands.
const tempDirVar = "{{TEMP_DIR}}"

// SetupSchema describes the work a test needs before Alloy starts.
type SetupSchema struct {
	// Files are written before Alloy starts.
	Files []SetupFileSchema `yaml:"files,omitempty"`
	// Commands run in order before Alloy starts.
	Commands []SetupCommandSchema `yaml:"commands,omitempty"`
}

// SetupFileSchema describes a file to create. Missing parent directories are
// created.
type SetupFileSchema struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content,omitempty"`
}

// SetupCommandSchema describes a command to run, without a shell.
type SetupCommandSchema struct {
	Run []string `yaml:"run"`
	// Cleanup runs after the test, even when the test fails or Run failed.
	Cleanup []string `yaml:"cleanup,omitempty"`
}

// setupResult is the outcome of running the setup of a test.
type setupResult struct {
	// config is the config with the placeholders replaced.
	config string
	// cleanup undoes the setup. It is safe to call when setup failed.
	cleanup func()
}

// runSetup creates the files and runs the commands of a test. A backslash in
// the working directory is doubled in the config, so a Windows path stays
// valid inside an Alloy string.
func runSetup(schema TestSchema, workDir string) (result setupResult, err error) {
	result.config = strings.ReplaceAll(string(schema.Config), tempDirVar, strings.ReplaceAll(workDir, `\`, `\\`))

	var cleanups []func()
	result.cleanup = func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	expand := func(s string) string { return strings.ReplaceAll(s, tempDirVar, workDir) }

	for _, file := range schema.Setup.Files {
		if file.Path == "" {
			return result, errors.New("setup file requires path")
		}
		path := filepath.FromSlash(expand(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return result, fmt.Errorf("create directory for %q: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o600); err != nil {
			return result, fmt.Errorf("write setup file %q: %w", path, err)
		}
	}

	for _, command := range schema.Setup.Commands {
		if len(command.Run) == 0 {
			return result, errors.New("setup command requires run")
		}

		// Register the cleanup first, so a command that fails halfway still
		// gets cleaned up.
		if len(command.Cleanup) > 0 {
			args := expandAll(command.Cleanup, expand)
			cleanups = append(cleanups, func() { _ = exec.Command(args[0], args[1:]...).Run() })
		}

		args := expandAll(command.Run, expand)
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			return result, fmt.Errorf("setup command %q failed: %w: %s", strings.Join(args, " "), err, out)
		}
	}

	return result, nil
}

func expandAll(args []string, expand func(string) string) []string {
	out := make([]string, len(args))
	for i, arg := range args {
		out[i] = expand(arg)
	}
	return out
}
