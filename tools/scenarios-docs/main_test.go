package scenariosdocs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testSHA = "0123456789abcdef0123456789abcdef01234567"

	githubPrefix = "https://github.com/grafana/alloy-scenarios/"

	// readmeTemplate has the same shape as the real README: a banner, an H1, an
	// intro, a preamble, one marked category whose single table row is
	// substituted, and a Contribute section.
	readmeTemplate = "<p align=\"center\">\n  <img src=\"https://example.invalid/b.png\"/>\n</p>\n\n" +
		"# Title\n\nIntro that is dropped.\n\n" +
		"## Before you begin\n\nPreamble text.\n\n" +
		"## Scenarios\n\nBrowse scenarios.\n\n" +
		"<!-- alloy-docs: slug=section -->\n\n### Section\n\nCategory prose.\n\n" +
		"| Scenario | Description |\n| -------- | ----------- |\n| %s | Row. |\n\n" +
		"## Contribute\n\nignored\n"
)

func readmeWith(row string) []byte {
	return []byte(fmt.Sprintf(readmeTemplate, row))
}

func pageBySlug(t *testing.T, pages []Page, slug string) Page {
	t.Helper()

	for _, p := range pages {
		if p.Slug == slug {
			return p
		}
	}
	t.Fatalf("no page with slug %q in %d pages", slug, len(pages))
	return Page{}
}

func TestTransformGolden(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("testdata", "README.md"))
	require.NoError(t, err)

	pages, err := Transform(readme, testSHA)
	require.NoError(t, err)

	slugs := make([]string, 0, len(pages))
	for _, p := range pages {
		slugs = append(slugs, p.Slug)
	}
	require.Equal(t, []string{preambleSlug, "logs", "opentelemetry-engine"}, slugs,
		"pages keep README order, with the preamble first")

	golden, err := os.ReadDir(filepath.Join("testdata", "golden"))
	require.NoError(t, err)
	require.Len(t, golden, len(pages), "every page has a golden file and vice versa")

	for _, p := range pages {
		t.Run(p.Slug, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", "golden", p.Slug+".md"))
			require.NoError(t, err)
			require.Equal(t, string(want), string(p.Content))
		})
	}
}

func TestTransformSplitsCategories(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("testdata", "README.md"))
	require.NoError(t, err)

	pages, err := Transform(readme, testSHA)
	require.NoError(t, err)

	preamble := string(pageBySlug(t, pages, preambleSlug).Content)
	require.Contains(t, preamble, "## Before you begin")
	require.Contains(t, preamble, "## Run a scenario")
	require.NotContains(t, preamble, "| Scenario |", "the preamble carries no tables")
	require.NotContains(t, preamble, "Refer to the [alloy-scenarios]", "the preamble has no scenario footer")

	logs := pageBySlug(t, pages, "logs")
	require.Equal(t, "Logs", logs.Title)
	require.Contains(t, string(logs.Content), "Tail log files with Alloy.")
	require.NotContains(t, string(logs.Content), "### Logs",
		"the category heading stays in the README; the page supplies the title")
	require.NotContains(t, string(logs.Content), "These scenarios focus on log collection",
		"the category prose stays in the README; the page supplies the intro")
	require.NotContains(t, string(logs.Content), "Drop health checks",
		"categories don't leak into each other")
}

func TestTransformRewritesLinks(t *testing.T) {
	tree := githubPrefix + "tree/main/"
	blob := githubPrefix + "blob/main/"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"directory link", "[A](logs-file/)", "[A]: " + tree + "logs-file"},
		{"nested directory link", "[A](otel-examples/cost-control/)", "[A]: " + tree + "otel-examples/cost-control"},
		{"fragment is kept", "[A](logs-file/#x)", "[A]: " + tree + "logs-file#x"},
		{"file link", "[A](LICENSE)", "[A]: " + blob + "LICENSE"},
		{"dot slash prefix", "[A](./logs-file/)", "[A]: " + tree + "logs-file"},
		{"absolute url", "[A](https://x.io/y)", "[A]: https://x.io/y"},
		{"anchor", "[A](#frag)", "[A]: #frag"},
		{"mailto", "[A](mailto:a@b.c)", "[A]: mailto:a@b.c"},
		{"site absolute path", "[A](/docs/x/)", "[A]: /docs/x/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages, err := Transform(readmeWith(tt.in), testSHA)
			require.NoError(t, err)

			page := string(pageBySlug(t, pages, "section").Content)
			require.Contains(t, page, "| [A][] | Row. |", "the link becomes a collapsed reference")
			require.Contains(t, page, tt.want, "the definition carries the destination")
			require.NotContains(t, page, "](", "no inline link survives")
		})
	}
}

func TestTransformLeavesImagesInline(t *testing.T) {
	pages, err := Transform(readmeWith("![A](https://x.io/i.png)"), testSHA)
	require.NoError(t, err)
	require.Contains(t, string(pageBySlug(t, pages, "section").Content), "![A](https://x.io/i.png)")
}

func TestTransformRejectsAmbiguousLinkText(t *testing.T) {
	readme := []byte("# T\n\n## Run\n\nx\n\n## Scenarios\n\n<!-- alloy-docs: slug=s -->\n\n### S\n\n" +
		"| A | B |\n| - | - |\n| [Same](one/) | [Same](two/) |\n\n## Contribute\n")

	_, err := Transform(readme, testSHA)
	require.ErrorContains(t, err, `link text "Same" points at both`)
}

func TestTransformDefinesLinksOnEveryPage(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("testdata", "README.md"))
	require.NoError(t, err)

	pages, err := Transform(readme, testSHA)
	require.NoError(t, err)

	// docs/shared renders each file separately, so a page that uses a label
	// must define it, and must not rely on another page's definitions.
	for _, p := range pages {
		t.Run(p.Slug, func(t *testing.T) {
			content := string(p.Content)
			for _, label := range usedLabels(content) {
				require.Contains(t, content, "\n["+label+"]: ",
					"page %q uses [%s][] and must define it", p.Slug, label)
			}
		})
	}

	logs := string(pageBySlug(t, pages, "logs").Content)
	require.NotContains(t, logs, "[Cost control]", "definitions don't leak between pages")
}

var usedLabelRe = regexp.MustCompile(`\[([^\]]+)\]\[\]`)

func usedLabels(content string) []string {
	var labels []string
	for _, m := range usedLabelRe.FindAllStringSubmatch(content, -1) {
		labels = append(labels, m[1])
	}
	return labels
}

func TestTransformLinksAreNotPinned(t *testing.T) {
	pages, err := Transform(readmeWith("[A](logs-file/)"), testSHA)
	require.NoError(t, err)

	section := string(pageBySlug(t, pages, "section").Content)
	require.Contains(t, section, githubPrefix+"tree/main/logs-file")
	require.NotContains(t, strings.SplitN(section, "-->", 2)[1], testSHA,
		"the commit appears only in the provenance comment, never in a link")
}

func TestTransformLeavesFencedCodeAlone(t *testing.T) {
	readme := []byte("# Title\n\n## Run\n\n```\n[a](logs-file/)\n```\n\n[B](logs-file/)\n\n" +
		"## Scenarios\n\n<!-- alloy-docs: slug=s -->\n\n### S\n\n" +
		"| A | B |\n| - | - |\n| x | y |\n\n## Contribute\n")

	pages, err := Transform(readme, testSHA)
	require.NoError(t, err)

	preamble := string(pageBySlug(t, pages, preambleSlug).Content)
	require.Contains(t, preamble, "```\n[a](logs-file/)\n```", "fenced code is untouched")
	require.Contains(t, preamble, "[B][]")
	require.Contains(t, preamble, "[B]: "+githubPrefix+"tree/main/logs-file")
}

func TestTransformAbsorbsExistingDefinitions(t *testing.T) {
	readme := []byte("# Title\n\n## Run\n\nUse [a][].\n\n[a]: logs-file/\n\n" +
		"## Scenarios\n\n<!-- alloy-docs: slug=s -->\n\n### S\n\n" +
		"| A | B |\n| - | - |\n| x | y |\n\n## Contribute\n")

	pages, err := Transform(readme, testSHA)
	require.NoError(t, err)

	preamble := string(pageBySlug(t, pages, preambleSlug).Content)
	require.Contains(t, preamble, "[a]: "+githubPrefix+"tree/main/logs-file")
	require.Equal(t, 1, strings.Count(preamble, "[a]: "), "the definition isn't duplicated")
}

func TestTransformAcceptsCRLF(t *testing.T) {
	lf := readmeWith("[a](logs-file/)")
	crlf := []byte(strings.ReplaceAll(string(lf), "\n", "\r\n"))

	want, err := Transform(lf, testSHA)
	require.NoError(t, err)
	got, err := Transform(crlf, testSHA)
	require.NoError(t, err)

	require.Len(t, got, len(want))
	for i := range want {
		require.Equal(t, string(want[i].Content), string(got[i].Content))
	}
}

func TestTransformErrors(t *testing.T) {
	replace := func(old, new string) []byte {
		return []byte(strings.Replace(string(readmeWith("[a](logs-file/)")), old, new, 1))
	}

	tests := []struct {
		name    string
		readme  []byte
		sha     string
		wantErr string
	}{
		{"link escapes the repository", readmeWith("[a](../x/)"), testSHA, "cannot resolve relative link"},
		{"relative image", readmeWith("![a](img/x.png)"), testSHA, "relative image"},
		{"hugo shortcode", readmeWith("{{< foo >}}"), testSHA, "shortcode delimiter"},
		{"missing end marker", replace("## Contribute", "## Other"), testSHA, "end marker"},
		{"missing scenarios heading", replace("## Scenarios", "## Other"), testSHA, `"## Scenarios" not found`},
		{"missing H1", []byte("## Scenarios\n\n## Contribute\n"), testSHA, "expected an H1"},
		{"missing H2", []byte("# Title\n\ntext only\n"), testSHA, "no H2 heading"},
		{"unterminated banner", []byte("<p align=\"center\">\n# Title\n"), testSHA, "unterminated"},
		{"sha is not hex", readmeWith("x"), "not-a-sha", "invalid commit SHA"},
		{"sha is uppercase", readmeWith("x"), strings.ToUpper(testSHA), "invalid commit SHA"},

		{
			"category without a marker",
			replace("<!-- alloy-docs: slug=section -->\n\n", ""),
			testSHA,
			`category "Section" has no alloy-docs slug marker`,
		},
		{
			"malformed marker",
			replace("<!-- alloy-docs: slug=section -->", "<!-- alloy-docs: slug=Section -->"),
			testSHA,
			"malformed alloy-docs marker",
		},
		{
			"marker without a category",
			replace("### Section", "Not a heading"),
			testSHA,
			`marker for slug "section" isn't followed by a category heading`,
		},
		{
			"no categories at all",
			[]byte("# T\n\n## Run\n\nx\n\n## Scenarios\n\nnothing here\n\n## Contribute\n"),
			testSHA,
			"no scenario categories found",
		},
		{
			"category without a table",
			replace("| Scenario | Description |\n| -------- | ----------- |\n| [a](logs-file/) | Row. |", "No table here."),
			testSHA,
			"no scenario table found",
		},
		{
			"table without rows",
			replace("\n| [a](logs-file/) | Row. |", ""),
			testSHA,
			"the scenario table has no rows",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Transform(tt.readme, tt.sha)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestTransformRejectsDuplicateSlugs(t *testing.T) {
	readme := []byte("# T\n\n## Run\n\nx\n\n## Scenarios\n\n" +
		"<!-- alloy-docs: slug=dup -->\n\n### First\n\n| A | B |\n| - | - |\n| x | y |\n\n" +
		"<!-- alloy-docs: slug=dup -->\n\n### Second\n\n| A | B |\n| - | - |\n| x | y |\n\n" +
		"## Contribute\n")

	_, err := Transform(readme, testSHA)
	require.ErrorContains(t, err, `duplicate alloy-docs slug "dup" on categories "First" and "Second"`)
}

func TestCommittedRevisionIsValid(t *testing.T) {
	require.NoError(t, validateSHA(revision), "revision.go holds a usable commit SHA")
}

func TestWriteRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revision.go")

	require.NoError(t, writeRevision(path, testSHA))
	require.Equal(t, testSHA, revisionIn(t, path))

	require.ErrorContains(t, writeRevision(path, "nope"), "invalid commit SHA")
	require.Equal(t, testSHA, revisionIn(t, path), "a rejected SHA leaves the file untouched")
}

// revisionIn returns the revision constant declared in path. Parsing the file
// as Go also proves that bump emits something that compiles.
func revisionIn(t *testing.T, path string) string {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	require.NoError(t, err)

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || value.Names[0].Name != "revision" {
				continue
			}
			lit, ok := value.Values[0].(*ast.BasicLit)
			require.True(t, ok, "revision is a literal")

			sha, err := strconv.Unquote(lit.Value)
			require.NoError(t, err)
			return sha
		}
	}

	t.Fatalf("no revision constant in %s", path)
	return ""
}

func TestFetchReadme(t *testing.T) {
	const body = "# hello\n"
	otherSHA := strings.Repeat("b", 40)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/grafana/alloy-scenarios/" + testSHA + "/README.md":
			_, _ = w.Write([]byte(body))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	got, err := fetchReadme(t.Context(), srv.Client(), srv.URL, testSHA)
	require.NoError(t, err)
	require.Equal(t, body, string(got))

	_, err = fetchReadme(t.Context(), srv.Client(), srv.URL, otherSHA)
	require.ErrorContains(t, err, "404")

	// A value that is not a SHA is rejected before any request is made.
	_, err = fetchReadme(t.Context(), srv.Client(), srv.URL, "../etc/passwd")
	require.ErrorContains(t, err, "invalid commit SHA")
}

func TestFetchReadmeRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxReadmeBytes+1))
	}))
	t.Cleanup(srv.Close)

	_, err := fetchReadme(t.Context(), srv.Client(), srv.URL, testSHA)
	require.ErrorContains(t, err, "larger than")
}

func TestWriteIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "out.md")

	changed, err := writeIfChanged(path, []byte("a"))
	require.NoError(t, err)
	require.True(t, changed, "first write creates the file and its directory")

	changed, err = writeIfChanged(path, []byte("a"))
	require.NoError(t, err)
	require.False(t, changed, "identical content is not rewritten")

	changed, err = writeIfChanged(path, []byte("b"))
	require.NoError(t, err)
	require.True(t, changed)
}

func TestVerifyPages(t *testing.T) {
	dir := t.TempDir()
	pages := []Page{{Slug: preambleSlug, Title: "Run a scenario"}, {Slug: "logs", Title: "Logs"}}

	err := verifyPages(dir, pages)
	require.ErrorContains(t, err, "_index.md is missing, for category \"Run a scenario\"")
	require.ErrorContains(t, err, "logs.md is missing, for category \"Logs\"")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "_index.md"),
		[]byte("intro\n\n"+SharedImport(preambleSlug)+"\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "logs.md"), []byte("intro only\n"), 0o644))

	err = verifyPages(dir, pages)
	require.ErrorContains(t, err, "logs.md doesn't import its table")
	require.NotContains(t, err.Error(), "_index.md")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "logs.md"),
		[]byte("intro\n\n"+SharedImport("logs")+"\n"), 0o644))
	require.NoError(t, verifyPages(dir, pages))
}

func TestPruneSharedDir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"logs.md", "stale.md", "keep.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644))
	}

	removed, err := pruneSharedDir(dir, []Page{{Slug: "logs"}})
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	require.ElementsMatch(t, []string{"logs.md", "keep.txt"}, names,
		"only generated Markdown files are pruned")
}

func TestSyncCommandWithSourceFile(t *testing.T) {
	dir := t.TempDir()

	sharedDir := filepath.Join(dir, "shared", "scenarios")
	pagesDir := filepath.Join(dir, "scenarios")
	require.NoError(t, os.MkdirAll(pagesDir, 0o755))

	run := func() (string, error) {
		var buf bytes.Buffer
		cmd := Command()
		cmd.SetOut(&buf)
		cmd.SetErr(&buf)
		cmd.SetArgs([]string{
			"sync",
			"--revision", testSHA,
			"--shared-dir", sharedDir,
			"--pages-dir", pagesDir,
			"--source-file", filepath.Join("testdata", "README.md"),
		})
		err := cmd.Execute()
		return buf.String(), err
	}

	// Without the hand-written pages the command refuses to write anything.
	_, err := run()
	require.ErrorContains(t, err, "every scenario category needs a page")
	require.NoDirExists(t, sharedDir)

	for slug, name := range map[string]string{
		preambleSlug:           "_index.md",
		"logs":                 "logs.md",
		"opentelemetry-engine": "opentelemetry-engine.md",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(pagesDir, name),
			[]byte("intro\n\n"+SharedImport(slug)+"\n"), 0o644))
	}

	out, err := run()
	require.NoError(t, err)
	require.Contains(t, out, "3 written")

	for _, slug := range []string{preambleSlug, "logs", "opentelemetry-engine"} {
		want, err := os.ReadFile(filepath.Join("testdata", "golden", slug+".md"))
		require.NoError(t, err)
		got, err := os.ReadFile(filepath.Join(sharedDir, slug+".md"))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got))
	}

	out, err = run()
	require.NoError(t, err)
	require.Contains(t, out, "already up to date")
}
