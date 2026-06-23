package metadata

import (
	"bytes"
	_ "embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
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
	fileNameGeneratedMetrics  = "generated_metrics.go"

	generatedDir         = "internal/metadata"
	generatedPackageName = "metadata"

	docsStartMarker = "<!-- START GENERATED METADATA -->"
	docsEndMarker   = "<!-- END GENERATED METADATA -->"
)

var (
	//go:embed templates/metadata.go.tmpl
	metadataTemplateSource string
	metadataTemplate       = template.Must(template.New("metadata").Parse(metadataTemplateSource))
	//go:embed templates/metadata.md.tmpl
	metadataDocsTemplateSource string
	metadataDocsTemplate       = template.Must(template.New("metadata-docs").Parse(metadataDocsTemplateSource))
	//go:embed templates/metrics.go.tmpl
	metricsTemplateSource string
	metricsTemplate       = template.Must(template.New("metrics").Funcs(template.FuncMap{
		"field": toPascalCase,
	}).Parse(metricsTemplateSource))
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
		mfile, err := os.Open(filepath.Join(dir, fileNameMetadata))
		if err != nil {
			return fmt.Errorf("failed to open metadata file: %w", err)
		}

		var metadata Metadata
		if err := yaml.NewDecoder(mfile).Decode(&metadata); err != nil {
			return fmt.Errorf("failed to decode metadata file: %w", err)
		}

		// Generate metadata go file
		if err := generateMetadataGoFile(dir, metadata); err != nil {
			return fmt.Errorf("failed to generate metadata file: %w", err)
		}

		// Generate metadata docs section
		if err := generateMetadataDocs(root, metadata); err != nil {
			return fmt.Errorf("failed to generate metadata docs: %w", err)
		}

		// Generate metrics go file
		if err := generateMetricsGoFile(dir, metadata); err != nil {
			return fmt.Errorf("failed to generate metrics file: %w", err)
		}
	}

	return nil
}

// writeGeneratedFile renders tmpl with data, gofmts it, and writes it to
// <dir>/internal/metadata/<fileName>, creating the directory if needed.
func writeGeneratedFile(dir, fileName string, tmpl *template.Template, data any) error {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to format generated source: %w", err)
	}

	outDir := filepath.Join(dir, generatedDir)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", outDir, err)
	}

	out := filepath.Join(outDir, fileName)
	if err := os.WriteFile(out, formatted, 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", out, err)
	}

	return nil
}

func generateMetricsGoFile(dir string, md Metadata) error {
	if len(md.Metrics) == 0 {
		return nil
	}

	type metricView struct {
		Name string
		Metric
	}

	names := make([]string, 0, len(md.Metrics))
	for name := range md.Metrics {
		names = append(names, name)
	}
	sort.Strings(names)

	metrics := make([]metricView, 0, len(names))
	for _, name := range names {
		m := md.Metrics[name]
		m.Help = strings.TrimSpace(m.Help)
		metrics = append(metrics, metricView{
			Name:   name,
			Metric: m,
		})
	}

	data := struct {
		Package   string
		Namespace string
		Subsystem string
		Metrics   []metricView
	}{
		Package:   generatedPackageName,
		Namespace: md.Namespace,
		Subsystem: md.Subsystem,
		Metrics:   metrics,
	}

	return writeGeneratedFile(dir, fileNameGeneratedMetrics, metricsTemplate, data)
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

func generateMetadataGoFile(dir string, md Metadata) error {
	type data struct {
		Package string
		Metadata
	}
	return writeGeneratedFile(dir, fileNameGeneratedMetadata, metadataTemplate, data{Package: generatedPackageName, Metadata: md})
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

func toPascalCase(s string) string {
	var b strings.Builder
	for p := range strings.SplitSeq(s, "_") {
		if p == "" {
			continue
		}

		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}
