package golint

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

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
		Use:   "go",
		Short: "Run golangci-lint",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f)
		},
	}

	f.RootFlag.Register(cmd)
	cmd.Flags().StringVar(&f.binary, "binary", "", "path to golangci-lint binary")

	return cmd
}

func run(f flags) error {
	root, err := f.RootFlag.Root()
	if err != nil {
		return err
	}

	result, err := discover.GoModFiles(root)
	if err != nil {
		return err
	}

	bin := "golangci-lint"
	if f.binary != "" {
		bin = f.binary
	}

	var errs []error
	for _, dir := range result.Dirs() {
		fmt.Println("Lint: ", dir)
		cmd := exec.Command(bin, "run", "-v", "--timeout=10m")
		cmd.Dir = dir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Errorf("lint %s", dir))
		}
	}

	return errors.Join(errs...)
}
