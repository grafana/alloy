package shelllint

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/internal/cli"
	"github.com/grafana/alloy/tools/internal/discover"
)

type flags struct {
	cli.RootFlag
}

func Command() *cobra.Command {
	var f flags
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Run shellcheck",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(f)
		},
	}

	f.RootFlag.Register(cmd)
	return cmd
}

func run(f flags) error {
	root, err := f.RootFlag.Root()
	if err != nil {
		return err
	}

	result, err := discover.Files(
		root,
		discover.MatchExtentionsFn(".sh", ".bash", ""),
		discover.WithSkipDirs("vendor", "node_modules"),
	)
	if err != nil {
		return err
	}

	var filesToCheck []string
	for _, f := range result.Files() {
		ok, err := hasShebang(f)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		filesToCheck = append(filesToCheck, f)
	}

	cmd := exec.Command("shellcheck", filesToCheck...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return errors.New("shellcheck failed")
	}

	return nil
}

var shellShebang = regexp.MustCompile(`^#!.*[/ ](sh|bash)(\s|$)`)

func hasShebang(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}

	s := bufio.NewScanner(f)
	if !s.Scan() {
		return false, s.Err()
	}

	return shellShebang.Match(s.Bytes()), nil
}
