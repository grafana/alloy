package scenariosdocs

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	defaultSharedDir = "docs/sources/shared/scenarios"
	defaultPagesDir  = "docs/sources/scenarios"

	// defaultRevisionFile is relative to the tools module, which is where the
	// command runs from.
	defaultRevisionFile = "scenarios-docs/revision.go"

	httpTimeout = 30 * time.Second
)

// Command returns the scenarios-docs command, which generates the Alloy
// scenarios documentation from the README of the alloy-scenarios repository.
func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scenarios-docs",
		Short: "Generate the Alloy scenarios documentation from the alloy-scenarios repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Usage()
		},
	}
	cmd.AddCommand(syncCommand(), bumpCommand())

	return cmd
}

func syncCommand() *cobra.Command {
	var sha, sharedDir, pagesDir, sourceFile string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Generate the shared scenario tables from the pinned alloy-scenarios revision",
		Long: `Downloads README.md from the pinned alloy-scenarios commit, converts each
scenario category into a shared documentation file, and writes the results to
--shared-dir only when the content changed.

Every category needs a hand-written page in --pages-dir that imports its
generated file, so that page titles, weights, and descriptions stay under human
review. The command fails when a page is missing.

The commit pins what the tables are generated from, not where they link to.
Scenario links point at the default branch, so they follow renames and never go
stale against an old snapshot.

Use --source-file to read the README from disk instead of downloading it.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSHA(sha); err != nil {
				return err
			}

			var readme []byte
			var err error
			if sourceFile != "" {
				readme, err = os.ReadFile(sourceFile)
				if err != nil {
					return fmt.Errorf("read source file %s: %w", sourceFile, err)
				}
			} else {
				client := &http.Client{Timeout: httpTimeout}
				readme, err = fetchReadme(commandContext(cmd), client, rawBaseURL, sha)
				if err != nil {
					return err
				}
			}

			pages, err := Transform(readme, sha)
			if err != nil {
				return fmt.Errorf("convert README at %s: %w", sha, err)
			}

			if err := verifyPages(pagesDir, pages); err != nil {
				return err
			}

			changed, err := writePages(sharedDir, pages)
			if err != nil {
				return err
			}
			removed, err := pruneSharedDir(sharedDir, pages)
			if err != nil {
				return err
			}

			switch {
			case changed == 0 && removed == 0:
				cmd.Printf("%s is already up to date, %d files\n", sharedDir, len(pages))
			default:
				cmd.Printf("Updated %s, %d written, %d removed\n", sharedDir, changed, removed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&sha, "revision", revision, "alloy-scenarios commit to generate from")
	cmd.Flags().StringVar(&sharedDir, "shared-dir", defaultSharedDir, "directory for the generated shared documentation files")
	cmd.Flags().StringVar(&pagesDir, "pages-dir", defaultPagesDir, "directory holding the hand-written scenario pages")
	cmd.Flags().StringVar(&sourceFile, "source-file", "", "read the README from this file instead of downloading it")

	return cmd
}

func bumpCommand() *cobra.Command {
	var revisionFile string

	cmd := &cobra.Command{
		Use:   "bump",
		Short: "Pin the current main commit of alloy-scenarios",
		Long: `Resolves the current commit of the alloy-scenarios main branch and rewrites the
revision constant with it. Run "sync" afterwards to regenerate the documentation.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			sha, err := resolveMainSHA(commandContext(cmd))
			if err != nil {
				return err
			}
			if err := writeRevision(revisionFile, sha); err != nil {
				return err
			}
			cmd.Printf("Pinned %s at %s\n", repoSlug, sha)
			return nil
		},
	}
	cmd.Flags().StringVar(&revisionFile, "revision-file", defaultRevisionFile, "path of the generated file that pins the alloy-scenarios commit")

	return cmd
}

// SharedImport returns the shortcode that a scenario page uses to import its
// generated table. The sync command checks for this exact string, so the pages
// and the generator can't drift apart.
func SharedImport(slug string) string {
	return fmt.Sprintf(`{{< docs/shared lookup="scenarios/%s.md" source="alloy" version="<ALLOY_VERSION>" >}}`, slug)
}

// pageFor returns the hand-written page that must import the given slug.
func pageFor(slug string) string {
	return slug + ".md"
}

// verifyPages checks that every generated file has a hand-written page that
// imports it.
func verifyPages(pagesDir string, pages []Page) error {
	var problems []string

	for _, p := range pages {
		path := filepath.Join(pagesDir, pageFor(p.Slug))

		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("%s is missing, for category %q", path, p.Title))
			continue
		}
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if !bytes.Contains(data, []byte(SharedImport(p.Slug))) {
			problems = append(problems, fmt.Sprintf("%s doesn't import its table, add: %s", path, SharedImport(p.Slug)))
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("every scenario category needs a page that imports its generated table:\n  %s",
			strings.Join(problems, "\n  "))
	}
	return nil
}

// writePages writes each generated page and reports how many changed.
func writePages(sharedDir string, pages []Page) (int, error) {
	changed := 0
	for _, p := range pages {
		wrote, err := writeIfChanged(filepath.Join(sharedDir, p.Slug+".md"), p.Content)
		if err != nil {
			return 0, err
		}
		if wrote {
			changed++
		}
	}
	return changed, nil
}

// pruneSharedDir removes generated files whose category no longer exists, so
// that a removed category doesn't leave an orphan page behind.
func pruneSharedDir(sharedDir string, pages []Page) (int, error) {
	entries, err := os.ReadDir(sharedDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", sharedDir, err)
	}

	want := make([]string, 0, len(pages))
	for _, p := range pages {
		want = append(want, p.Slug+".md")
	}
	sort.Strings(want)

	removed := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || slices.Contains(want, name) {
			continue
		}
		if err := os.Remove(filepath.Join(sharedDir, name)); err != nil {
			return 0, fmt.Errorf("remove %s: %w", filepath.Join(sharedDir, name), err)
		}
		removed++
	}
	return removed, nil
}

// commandContext returns the command's context, or a background context when
// the command was not started through ExecuteContext.
func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// writeIfChanged writes data to path unless the file already has exactly that
// content. It reports whether the file was written.
func writeIfChanged(path string, data []byte) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}
	if err == nil && bytes.Equal(existing, data) {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}
