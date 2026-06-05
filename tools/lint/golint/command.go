package golint

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/internal/cli"
	"github.com/grafana/alloy/tools/internal/discover"
)

type flags struct {
	cli.RootFlag
	binary string
}

func Command() *cobra.Command {
	var f flags
	cmd := &cobra.Command{
		Use:   "go [targets...]",
		Short: "Run golangci-lint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f, args)
		},
	}

	f.RootFlag.Register(cmd)
	cmd.Flags().StringVar(&f.binary, "binary", "", "path to golangci-lint binary")

	return cmd
}

func run(f flags, targets []string) error {
	root, err := f.RootFlag.Root()
	if err != nil {
		return err
	}

	bin := "golangci-lint"
	if f.binary != "" {
		bin = f.binary
	}

	result, err := discover.GoModFiles(root)
	if err != nil {
		return err
	}
	dirs := result.Dirs()
	targetsByDir, err := groupTargetsByDir(root, dirs, targets)
	if err != nil {
		return err
	}

	var errs []error
	for _, dir := range dirs {
		targets, ok := targetsByDir[dir]
		if len(targetsByDir) > 0 && !ok {
			continue
		}
		fmt.Println("Lint: ", dir)
		args := append([]string{"run", "-v", "--timeout=10m"}, targets...)
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Errorf("lint %s: %w", dir, err))
		}
	}

	return errors.Join(errs...)
}

func groupTargetsByDir(root string, dirs []string, targets []string) (map[string][]string, error) {
	if len(targets) == 0 {
		return nil, nil
	}

	targetsByDir := make(map[string][]string)
	for _, target := range targets {
		dir, moduleTarget, err := resolveModuleTarget(root, dirs, target)
		if err != nil {
			return nil, err
		}
		targetsByDir[dir] = append(targetsByDir[dir], moduleTarget)
	}
	return targetsByDir, nil
}

func resolveModuleTarget(root string, dirs []string, target string) (string, string, error) {
	cleanTarget := filepath.Clean(filepath.FromSlash(target))
	absTarget := cleanTarget
	if !filepath.IsAbs(absTarget) {
		absTarget = filepath.Join(root, cleanTarget)
	}

	dir := containingModuleDir(dirs, absTarget)
	if dir == "" {
		return "", "", fmt.Errorf("target %q is not in a Go module under %q", target, root)
	}

	moduleTarget, err := filepath.Rel(dir, absTarget)
	if err != nil {
		return "", "", fmt.Errorf("resolve target %q relative to module %q: %w", target, dir, err)
	}
	return dir, moduleTarget, nil
}

func containingModuleDir(dirs []string, target string) string {
	var match string
	for _, dir := range dirs {
		if target == dir || strings.HasPrefix(target, dir+string(filepath.Separator)) {
			if len(dir) > len(match) {
				match = dir
			}
		}
	}
	return match
}
