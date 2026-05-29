package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/grafana/alloy/tools/internal/git"
)

type RootFlag struct{ root string }

func (r *RootFlag) Register(cmd *cobra.Command) {
	cmd.Flags().StringVar(&r.root, "root", "", "repository root (default: git root)")
}

func (r *RootFlag) Root() (string, error) {
	if r.root == "" {
		return git.Root()
	}
	abs, err := filepath.Abs(r.root)
	if err != nil {
		return "", fmt.Errorf("resolve root %q: %w", r.root, err)
	}
	return abs, nil
}
