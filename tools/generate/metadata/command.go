package metadata

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/grafana/alloy/tools/internal/cli"
	"github.com/grafana/alloy/tools/internal/discover"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const (
	fileNameMetadata          = "metadata.yml"
	fileNameGeneratedMetadata = "generated_metadata.go"
)

//go:embed templates/metadata.go.tmpl
var metadataTemplateSource string

var metadataTemplate = template.Must(template.New("metadata").Parse(metadataTemplateSource))

//go:embed templates/metadata.md.tmpl
var metadataDocsTemplateSource string

var metadataDocsTemplate = template.Must(template.New("metadata-docs").Parse(metadataDocsTemplateSource))

const (
	docsStartMarker = "<!-- START GENERATED METADATA -->"
	docsEndMarker   = "<!-- END GENERATED METADATA -->"
)

var platformDisplayNames = map[Platform]string{
	PlatformLinux:   "Linux",
	PlatformDarwin:  "macOS",
	PlatformWindows: "Windows",
	PlatformFreeBSD: "FreeBSD",
}

type flags struct {
	cli.RootFlag
}

func Command() *cobra.Command {
	var f flags
	cmd := &cobra.Command{
		Use:   "metadata",
		Short: "Generate component metadata",
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

	result, err := discover.Files(root, discover.MatchPatternFn(fileNameMetadata))
	if err != nil {
		return err
	}

	for _, dir := range result.Dirs() {
		pkg, err := parsePkg(dir)
		if err != nil {
			return err
		}

		mfile, err := os.Open(filepath.Join(dir, fileNameMetadata))
		if err != nil {
			return fmt.Errorf("failed to open metadata file: %w", err)
		}

		var metadata Metadata

		if err := yaml.NewDecoder(mfile).Decode(&metadata); err != nil {
			return fmt.Errorf("failed to decode metadata file: %w", err)
		}

		fmt.Printf("package %s: %+v\n", pkg, metadata)

		// Generate metadata go file
		if err := generateMetadataGoFile(pkg, dir, metadata); err != nil {
			return fmt.Errorf("failed to generate metadata file: %w", err)
		}

		// Generate metadata docs section
		if err := generateMetadataDocs(root, metadata); err != nil {
			return fmt.Errorf("failed to generate metadata docs: %w", err)
		}
	}

	return nil
}

func generateMetadataDocs(root string, md Metadata) error {
	type data struct {
		Platforms    []string
		Requirements []Requirement
	}

	var d data

	for _, p := range md.Platforms {
		name := platformDisplayNames[p]
		if name == "" {
			name = string(p)
		}
		d.Platforms = append(d.Platforms, name)
	}

	for _, r := range md.Requirements {
		d.Requirements = append(d.Requirements, Requirement{
			Description: strings.TrimSpace(r.Description),
			Reference:   strings.TrimSpace(r.Reference),
		})
	}

	var buf bytes.Buffer
	if err := metadataDocsTemplate.Execute(&buf, d); err != nil {
		return fmt.Errorf("failed to execute docs template: %w", err)
	}

	group, _, _ := strings.Cut(md.Name, ".")
	docPath := filepath.Join(root, "docs", "sources", "reference", "components", group, md.Name+".md")

	return writeBetweenMarkers(docPath, docsStartMarker, docsEndMarker, buf.String())
}

func generateMetadataGoFile(pkg, dir string, md Metadata) error {
	type data struct {
		Package string
		Metadata
	}

	d := data{
		Package:  pkg,
		Metadata: md,
	}

	var buf bytes.Buffer
	if err := metadataTemplate.Execute(&buf, d); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated source: %w", err)
	}

	out := filepath.Join(dir, fileNameGeneratedMetadata)
	if err := os.WriteFile(out, formatted, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", out, err)
	}

	return nil
}

func parsePkg(dir string) (string, error) {
	cmd := exec.Command("go", "list", "-f", "{{.Name}}")
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("failed to resolve package name in %s: %s", dir, exitErr.Stderr)
		}
		return "", fmt.Errorf("failed to resolve package name in %s: %w", dir, err)
	}

	return strings.TrimSpace(string(out)), nil
}

func writeBetweenMarkers(path, start, end, content string) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	startIdx := bytes.Index(contents, []byte(start))
	endIdx := bytes.LastIndex(contents, []byte(end))
	if startIdx == -1 || endIdx == -1 {
		return fmt.Errorf("markers %q and %q not found in %s", start, end, path)
	}

	var out []byte
	out = append(out, contents[:startIdx]...)
	out = append(out, start...)
	out = append(out, content...)
	out = append(out, end...)
	out = append(out, contents[endIdx+len(end):]...)

	return os.WriteFile(path, out, 0o644)
}
