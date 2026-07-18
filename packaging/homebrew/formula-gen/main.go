// Command formula-gen renders the Grafana Alloy Homebrew formula
// (alloy.rb.tpl) into a concrete alloy.rb for the homebrew-grafana tap.
//
// It fills the template with the release version, the per-OS/arch artifact file
// names (from a JSON file), and the SHA256 checksums fetched from the release's
// SHA256SUMS asset. The program depends only on the Go standard library.
//
// Usage:
//
//	go run -C packaging/homebrew/formula-gen/ . -tag v1.17.1 -artifacts ../artifacts.json -out alloy.rb
//
// Required environment variables:
//
//	GITHUB_SERVER_URL   e.g. https://github.com
//	GITHUB_REPOSITORY   e.g. grafana/alloy
//
// The SHA256SUMS asset is fetched anonymously: grafana/alloy is public and the
// bump workflow only runs on published (non-draft) releases.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
)

// newFlagSet returns the program's flag set. It uses ContinueOnError so run can
// return parse errors instead of calling os.Exit.
func newFlagSet() *flag.FlagSet {
	return flag.NewFlagSet("formula-gen", flag.ContinueOnError)
}

// artifact describes a single released binary and its archive.
type artifact struct {
	// Package is the release asset (zip) file name, e.g. alloy-darwin-arm64.zip.
	Package string `json:"package"`
	// BinFile is the binary file name inside the archive, e.g. alloy-darwin-arm64.
	BinFile string `json:"binFile"`
	// Checksum is the SHA256 of Package. Populated from the SHA256SUMS asset,
	// not from the artifacts JSON.
	Checksum string `json:"-"`
}

// archMap holds the artifacts for the two supported architectures.
type archMap struct {
	Arm64 artifact `json:"arm64"`
	Amd64 artifact `json:"amd64"`
}

// artifacts holds the artifacts for the two supported operating systems. Field
// names are PascalCase to match the template (.Artifacts.Darwin.Arm64.Package).
type artifacts struct {
	Darwin archMap `json:"darwin"`
	Linux  archMap `json:"linux"`
}

// all returns every artifact by pointer so callers can fill checksums and
// validate in a single pass.
func (a *artifacts) all() []*artifact {
	return []*artifact{
		&a.Darwin.Arm64, &a.Darwin.Amd64,
		&a.Linux.Arm64, &a.Linux.Amd64,
	}
}

// templateData is interpolated into alloy.rb.tpl.
type templateData struct {
	// Version is Tag without the leading "v", e.g. 1.17.1.
	Version string
	// Tag is the raw git tag, e.g. v1.17.1.
	Tag string
	// BaseURL is the release download base, e.g.
	// https://github.com/grafana/alloy/releases/download.
	BaseURL string
	// Artifacts holds the per-OS/arch package names and checksums.
	Artifacts artifacts
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := newFlagSet()

	var (
		tag          string
		artifactsPth string
		templatePth  string
		out          string
	)
	fs.StringVar(&tag, "tag", "", "Raw git tag, e.g. v1.17.1 (required)")
	fs.StringVar(&artifactsPth, "artifacts", "../artifacts.json", "Path to the artifacts JSON file")
	fs.StringVar(&templatePth, "template", "../alloy.rb.tpl", "Path to the formula template")
	fs.StringVar(&out, "out", "alloy.rb", "Output file path")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if tag == "" {
		return fmt.Errorf("flag -tag is required")
	}

	serverURL, err := requireEnv("GITHUB_SERVER_URL")
	if err != nil {
		return err
	}
	repository, err := requireEnv("GITHUB_REPOSITORY")
	if err != nil {
		return err
	}
	data := templateData{
		Tag:     tag,
		Version: strings.TrimPrefix(tag, "v"),
		BaseURL: fmt.Sprintf("%s/%s/releases/download",
			strings.TrimRight(serverURL, "/"), strings.Trim(repository, "/")),
	}

	if err := loadArtifacts(artifactsPth, &data.Artifacts); err != nil {
		return err
	}

	sumsURL := fmt.Sprintf("%s/%s/SHA256SUMS", data.BaseURL, tag)
	sums, err := fetchChecksums(sumsURL)
	if err != nil {
		return fmt.Errorf("fetching checksums: %w", err)
	}
	if err := applyChecksums(&data.Artifacts, sums); err != nil {
		return err
	}

	rendered, err := render(templatePth, data)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, rendered, 0o644); err != nil {
		return fmt.Errorf("writing formula to %s: %w", out, err)
	}
	return nil
}

// loadArtifacts reads and decodes the artifacts JSON file.
func loadArtifacts(path string, dst *artifacts) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading artifacts file: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("parsing artifacts file: %w", err)
	}
	for _, a := range dst.all() {
		if a.Package == "" || a.BinFile == "" {
			return fmt.Errorf("artifacts file is missing package/binFile for one or more OS/arch entries")
		}
	}
	return nil
}

// fetchChecksums downloads and parses a SHA256SUMS file, returning a map from
// file name to hex checksum. The file uses GNU coreutils sha256sum output:
// "<hash>  <name>" (text mode) or "<hash> *<name>" (binary mode).
func fetchChecksums(url string) (map[string]string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GET %s: unexpected status %s: %s", url, resp.Status, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseChecksums(string(body)), nil
}

// parseChecksums parses the contents of a SHA256SUMS file.
func parseChecksums(contents string) map[string]string {
	sums := make(map[string]string)
	for line := range strings.SplitSeq(contents, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*") // strip binary-mode marker
		sums[name] = fields[0]
	}
	return sums
}

// applyChecksums fills every artifact's Checksum from sums, keyed by Package.
func applyChecksums(a *artifacts, sums map[string]string) error {
	var missing []string
	for _, art := range a.all() {
		sum, ok := sums[art.Package]
		if !ok {
			missing = append(missing, art.Package)
			continue
		}
		if !isSHA256(sum) {
			return fmt.Errorf("checksum for %s is not a valid SHA256: %q", art.Package, sum)
		}
		art.Checksum = sum
	}
	if len(missing) > 0 {
		return fmt.Errorf("SHA256SUMS is missing checksums for: %s", strings.Join(missing, ", "))
	}
	return nil
}

// isSHA256 reports whether s is a 64-character lowercase/uppercase hex string.
func isSHA256(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9',
			r >= 'a' && r <= 'f',
			r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// render parses the template at path and executes it against data.
func render(path string, data templateData) ([]byte, error) {
	tmpl, err := template.New("formula").
		Option("missingkey=error").
		ParseFiles(path)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	// ParseFiles names the template after the file's base name.
	if err := tmpl.ExecuteTemplate(&buf, baseName(path), data); err != nil {
		return nil, fmt.Errorf("rendering template: %w", err)
	}
	return buf.Bytes(), nil
}

func baseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}

func requireEnv(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return v, nil
}
