package syncreplaces

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/modfile"
)

const (
	builderConfigSharedReplacesStart = "<BEGIN_SHARED_REPLACE_DIRECTIVES>"
	builderConfigSharedReplacesEnd   = "<END_SHARED_REPLACE_DIRECTIVES>"
	syncedCommentSuffix              = " (synced from collector/builder-config.yaml)"
)

type replaceEntry struct {
	Comments []string
	Value    string
}

func Command() *cobra.Command {
	var builderConfigPath string
	var goModPath string

	cmd := &cobra.Command{
		Use:   "sync-replaces",
		Short: "Sync shared builder config replace directives into go.mod",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := syncBuilderConfigReplacesToGoMod(builderConfigPath, goModPath); err != nil {
				return err
			}
			return runGoModTidy(filepath.Dir(goModPath))
		},
	}
	cmd.Flags().StringVar(&builderConfigPath, "builder-config", "collector/builder-config.yaml", "path to the canonical OCB builder config")
	cmd.Flags().StringVar(&goModPath, "go-mod", "go.mod", "path to the root go.mod file to update")

	return cmd
}

func syncBuilderConfigReplacesToGoMod(builderConfigPath, goModPath string) error {
	builderConfig, err := os.ReadFile(builderConfigPath)
	if err != nil {
		return fmt.Errorf("read builder config %s: %w", builderConfigPath, err)
	}

	replaces, err := extractSharedReplaces(builderConfig)
	if err != nil {
		return fmt.Errorf("extract shared replaces from %s: %w", builderConfigPath, err)
	}

	goMod, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("read go.mod %s: %w", goModPath, err)
	}

	updated, err := syncGoModReplaces(goModPath, goMod, replaces)
	if err != nil {
		return fmt.Errorf("sync replaces in %s: %w", goModPath, err)
	}

	mode := os.FileMode(0o644)
	if fi, err := os.Stat(goModPath); err == nil {
		mode = fi.Mode()
	}
	if err := os.MkdirAll(filepath.Dir(goModPath), 0o755); err != nil {
		return fmt.Errorf("create go.mod directory: %w", err)
	}
	if err := os.WriteFile(goModPath, updated, mode); err != nil {
		return fmt.Errorf("write go.mod %s: %w", goModPath, err)
	}

	return nil
}

func extractSharedReplaces(builderConfig []byte) ([]replaceEntry, error) {
	scanner := newBuilderConfigScanner(builderConfig)
	if err := scanner.findSharedReplacesStart(); err != nil {
		return nil, err
	}

	var replaces []replaceEntry
	for {
		entry, err := scanner.readSharedReplaceEntry()
		if err != nil {
			return nil, err
		}
		if entry == nil {
			return replaces, nil
		}
		replaces = append(replaces, *entry)
	}
}

func newReplaceEntry(value string, comments []string) (replaceEntry, error) {
	if len(comments) == 0 {
		return replaceEntry{}, fmt.Errorf("shared replace %q must have a comment", value)
	}

	return replaceEntry{
		Comments: append([]string(nil), comments...),
		Value:    strings.TrimSpace(value),
	}, nil
}

func moduleVersionString(path, version string) string {
	if version == "" {
		return path
	}
	return path + " " + version
}

func isLocalReplacement(path string) bool {
	return strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/")
}

func syncGoModReplaces(filename string, data []byte, replaces []replaceEntry) ([]byte, error) {
	parsed, err := modfile.Parse(filename, data, nil)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}

	for _, replace := range append([]*modfile.Replace(nil), parsed.Replace...) {
		if isLocalReplacement(replace.New.Path) {
			continue
		}
		if replace.Syntax != nil {
			replace.Syntax.Before = nil
			replace.Syntax.Suffix = nil
			replace.Syntax.After = nil
		}
		if err := parsed.DropReplace(replace.Old.Path, replace.Old.Version); err != nil {
			return nil, fmt.Errorf("drop replace %s: %w", moduleVersionString(replace.Old.Path, replace.Old.Version), err)
		}
	}

	formatted, err := parsed.Format()
	if err != nil {
		return nil, fmt.Errorf("format go.mod: %w", err)
	}
	return appendSharedReplaces(formatted, replaces), nil
}

func appendSharedReplaces(data []byte, replaces []replaceEntry) []byte {
	if len(replaces) == 0 {
		return data
	}

	var builder strings.Builder
	builder.WriteString(strings.TrimRight(string(data), "\n"))
	builder.WriteString("\n\n")
	for _, replace := range replaces {
		for _, comment := range replace.Comments {
			builder.WriteString("// ")
			builder.WriteString(withSyncedCommentSuffix(comment))
			builder.WriteString("\n")
		}
		builder.WriteString("replace ")
		builder.WriteString(replace.Value)
		builder.WriteString("\n\n")
	}
	return []byte(builder.String())
}

func withSyncedCommentSuffix(comment string) string {
	comment = strings.TrimSpace(comment)
	if strings.HasSuffix(comment, syncedCommentSuffix) {
		return comment
	}
	return comment + syncedCommentSuffix
}
