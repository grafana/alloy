package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
	"golang.org/x/mod/semver"
)

type depSpec struct {
	// Human-friendly name.
	name string
	// GitHub repo slug: owner/repo
	slug string
	// Normalize input versions/refs to something usable against the repo.
	// For most deps this is the identity function. For Prometheus it converts
	// Go module versions like v0.308.0 -> v3.8.0.
	normalizeRef func(string) (string, error)
}

func main() {
	var (
		dep  = flag.String("dep", "", "Key dependency alias or module path")
		from = flag.String("from", "", "Version/tag/sha you are updating from")
		to   = flag.String("to", "", "Version/tag/sha you are updating to")
		max  = flag.Int("max-releases", 50, "Maximum number of releases to print")
	)
	flag.Parse()

	if *dep == "" || *from == "" || *to == "" {
		log.Fatal("--dep, --from and --to are required")
	}

	spec, err := resolveDep(*dep)
	if err != nil {
		log.Fatalf("unsupported dependency %q: %v", *dep, err)
	}

	fromRef, err := spec.normalizeRef(strings.TrimSpace(*from))
	if err != nil {
		log.Fatalf("failed to normalize --from: %v", err)
	}
	toRef, err := spec.normalizeRef(strings.TrimSpace(*to))
	if err != nil {
		log.Fatalf("failed to normalize --to: %v", err)
	}

	owner, repo, err := splitSlug(spec.slug)
	if err != nil {
		log.Fatalf("invalid repo slug %q: %v", spec.slug, err)
	}

	ctx := context.Background()
	client := newGitHubClient(ctx)

	fmt.Printf("# Dependency changelog\n\n")
	fmt.Printf("- **Dependency**: %s\n", spec.name)
	fmt.Printf("- **Repo**: %s\n", spec.slug)
	fmt.Printf("- **From**: %s\n", fromRef)
	fmt.Printf("- **To**: %s\n\n", toRef)

	// Try to print releases if from/to are semver tags.
	printedReleases := false
	if semver.IsValid(ensureV(fromRef)) && semver.IsValid(ensureV(toRef)) {
		err = printReleaseNotesBetween(ctx, client, owner, repo, ensureV(fromRef), ensureV(toRef), *max)
		if err != nil {
			fmt.Printf("\n> Note: failed to fetch releases (%v). Falling back to compare output.\n\n", err)
		} else {
			printedReleases = true
		}
	}

	// Always provide a compare summary as a backstop, unless releases were printed
	// and the caller is already getting sufficient detail.
	// (Release notes can be missing or incomplete; compare is still useful.)
	if !printedReleases {
		if err := printCompareSummary(ctx, client, owner, repo, fromRef, toRef); err != nil {
			log.Fatalf("failed to fetch compare summary: %v", err)
		}
	}
}

func resolveDep(dep string) (depSpec, error) {
	dep = strings.TrimSpace(dep)
	depLower := strings.ToLower(dep)

	// Aliases.
	aliases := map[string]depSpec{
		"otelcol": {
			name:         "OpenTelemetry Collector (core)",
			slug:         "open-telemetry/opentelemetry-collector",
			normalizeRef: normalizeDefault,
		},
		"otelcol-contrib": {
			name:         "OpenTelemetry Collector Contrib",
			slug:         "open-telemetry/opentelemetry-collector-contrib",
			normalizeRef: normalizeDefault,
		},
		"prometheus": {
			name:         "Prometheus",
			slug:         "prometheus/prometheus",
			normalizeRef: normalizePrometheus,
		},
		"prom-common": {
			name:         "Prometheus common",
			slug:         "prometheus/common",
			normalizeRef: normalizeDefault,
		},
		"prom-client-golang": {
			name:         "Prometheus client_golang",
			slug:         "prometheus/client_golang",
			normalizeRef: normalizeDefault,
		},
		"prom-client-model": {
			name:         "Prometheus client_model",
			slug:         "prometheus/client_model",
			normalizeRef: normalizeDefault,
		},
		"beyla": {
			name:         "Beyla",
			slug:         "grafana/beyla",
			normalizeRef: normalizeDefault,
		},
		"loki": {
			name:         "Loki",
			slug:         "grafana/loki",
			normalizeRef: normalizeDefault,
		},
		// These map to the forks Alloy actually uses.
		"obi": {
			name:         "OpenTelemetry eBPF instrumentation (Grafana fork)",
			slug:         "grafana/opentelemetry-ebpf-instrumentation",
			normalizeRef: normalizeDefault,
		},
		"ebpf-profiler": {
			name:         "OpenTelemetry eBPF profiler (Grafana fork)",
			slug:         "grafana/opentelemetry-ebpf-profiler",
			normalizeRef: normalizeDefault,
		},
	}
	if spec, ok := aliases[depLower]; ok {
		return spec, nil
	}

	// Module paths.
	// Note: multiple modules live in the same upstream repo; we map by prefix.
	prefixes := []struct {
		prefix string
		spec   depSpec
	}{
		{
			prefix: "go.opentelemetry.io/collector",
			spec: depSpec{
				name:         "OpenTelemetry Collector (core)",
				slug:         "open-telemetry/opentelemetry-collector",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "github.com/open-telemetry/opentelemetry-collector-contrib",
			spec: depSpec{
				name:         "OpenTelemetry Collector Contrib",
				slug:         "open-telemetry/opentelemetry-collector-contrib",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "github.com/prometheus/prometheus",
			spec: depSpec{
				name:         "Prometheus",
				slug:         "prometheus/prometheus",
				normalizeRef: normalizePrometheus,
			},
		},
		{
			prefix: "github.com/prometheus/common",
			spec: depSpec{
				name:         "Prometheus common",
				slug:         "prometheus/common",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "github.com/prometheus/client_golang",
			spec: depSpec{
				name:         "Prometheus client_golang",
				slug:         "prometheus/client_golang",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "github.com/prometheus/client_model",
			spec: depSpec{
				name:         "Prometheus client_model",
				slug:         "prometheus/client_model",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "github.com/grafana/beyla",
			spec: depSpec{
				name:         "Beyla",
				slug:         "grafana/beyla",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "github.com/grafana/loki",
			spec: depSpec{
				name:         "Loki",
				slug:         "grafana/loki",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "go.opentelemetry.io/obi",
			spec: depSpec{
				name:         "OpenTelemetry eBPF instrumentation (Grafana fork)",
				slug:         "grafana/opentelemetry-ebpf-instrumentation",
				normalizeRef: normalizeDefault,
			},
		},
		{
			prefix: "go.opentelemetry.io/ebpf-profiler",
			spec: depSpec{
				name:         "OpenTelemetry eBPF profiler (Grafana fork)",
				slug:         "grafana/opentelemetry-ebpf-profiler",
				normalizeRef: normalizeDefault,
			},
		},
	}

	for _, p := range prefixes {
		if strings.HasPrefix(dep, p.prefix) {
			return p.spec, nil
		}
	}

	return depSpec{}, fmt.Errorf("unknown key dependency (try an alias like otelcol, otelcol-contrib, prometheus, loki, beyla, prom-common, prom-client-golang, prom-client-model, obi, ebpf-profiler)")
}

func newGitHubClient(ctx context.Context) *github.Client {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc := oauth2.NewClient(ctx, ts)
		return github.NewClient(tc)
	}

	// Anonymous client (rate-limited but works for small queries).
	return github.NewClient(&http.Client{Timeout: 30 * time.Second})
}

func splitSlug(slug string) (string, string, error) {
	parts := strings.Split(slug, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected owner/repo")
	}
	return parts[0], parts[1], nil
}

func ensureV(s string) string {
	if s == "" {
		return s
	}
	if s[0] == 'v' {
		return s
	}
	// If it looks like a version, prefix with v.
	if (s[0] >= '0' && s[0] <= '9') || strings.HasPrefix(s, "0.") {
		return "v" + s
	}
	return s
}

func normalizeDefault(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty ref")
	}
	// If this is a pseudo-version, try to use its commit hash.
	if sha := tryExtractPseudoSHA(s); sha != "" {
		return sha, nil
	}
	return s, nil
}

// Prometheus is special: Alloy uses Go module versions like v0.308.0, but the
// upstream release tags are v3.8.0.
func normalizePrometheus(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty ref")
	}
	// If caller already passed a real Prometheus release tag, accept it.
	if semver.IsValid(ensureV(s)) {
		v := ensureV(s)
		// If it is already v3.x.y (or v2.x.y), keep it.
		if strings.HasPrefix(v, "v2.") || strings.HasPrefix(v, "v3.") || strings.HasPrefix(v, "v4.") {
			return v, nil
		}
	}
	// Otherwise treat as module version v0.<n>.<p> and convert.
	return promTagFromModuleVersion(s)
}

var pseudoSHARe = regexp.MustCompile(`(?i)-([0-9a-f]{7,40})$`)

func tryExtractPseudoSHA(version string) string {
	m := pseudoSHARe.FindStringSubmatch(version)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

func promTagFromModuleVersion(moduleVer string) (string, error) {
	v := ensureV(moduleVer)
	// Expected: v0.<n>.<p>
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("unexpected Prometheus module version %q (expected v0.<n>.<p>)", moduleVer)
	}
	if parts[0] != "0" {
		return "", fmt.Errorf("unexpected Prometheus module version %q (expected major v0)", moduleVer)
	}

	n, err := atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid Prometheus module minor %q: %w", parts[1], err)
	}
	patch, err := atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid Prometheus module patch %q: %w", parts[2], err)
	}

	major := n / 100
	minor := n % 100
	return fmt.Sprintf("v%d.%d.%d", major, minor, patch), nil
}

func atoi(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("non-digit %q", r)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

type releaseNote struct {
	Tag         string
	Name        string
	PublishedAt time.Time
	Body        string
}

func printReleaseNotesBetween(ctx context.Context, client *github.Client, owner, repo, fromTag, toTag string, max int) error {
	fromTag = ensureV(fromTag)
	toTag = ensureV(toTag)

	if !semver.IsValid(fromTag) || !semver.IsValid(toTag) {
		return fmt.Errorf("from/to must be valid semver tags (got %q -> %q)", fromTag, toTag)
	}
	if semver.Compare(fromTag, toTag) > 0 {
		return fmt.Errorf("from must be <= to (%s > %s)", fromTag, toTag)
	}

	opts := &github.ListOptions{PerPage: 100}
	var notes []releaseNote

	for {
		rels, resp, err := client.Repositories.ListReleases(ctx, owner, repo, opts)
		if err != nil {
			return err
		}

		for _, r := range rels {
			tag := ensureV(r.GetTagName())
			if !semver.IsValid(tag) {
				continue
			}
			if semver.Compare(tag, fromTag) <= 0 {
				continue
			}
			if semver.Compare(tag, toTag) > 0 {
				continue
			}

			var published time.Time
			if r.PublishedAt != nil {
				published = r.GetPublishedAt().Time
			}
			notes = append(notes, releaseNote{
				Tag:         tag,
				Name:        strings.TrimSpace(r.GetName()),
				PublishedAt: published,
				Body:        strings.TrimSpace(r.GetBody()),
			})
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
		if opts.Page > 20 {
			// Safety valve.
			break
		}
	}

	if len(notes) == 0 {
		return fmt.Errorf("no GitHub releases found between %s and %s", fromTag, toTag)
	}

	// Sort ascending by version.
	sort.Slice(notes, func(i, j int) bool {
		return semver.Compare(notes[i].Tag, notes[j].Tag) < 0
	})

	if max > 0 && len(notes) > max {
		notes = notes[len(notes)-max:]
	}

	fmt.Printf("## Releases (%d)\n\n", len(notes))
	for _, n := range notes {
		title := n.Tag
		if n.Name != "" {
			title = fmt.Sprintf("%s — %s", n.Tag, n.Name)
		}
		if !n.PublishedAt.IsZero() {
			title = fmt.Sprintf("%s (%s)", title, n.PublishedAt.Format("2006-01-02"))
		}
		fmt.Printf("### %s\n\n", title)
		if n.Body == "" {
			fmt.Printf("(no release notes body)\n\n")
			continue
		}
		fmt.Println(n.Body)
		if !strings.HasSuffix(n.Body, "\n") {
			fmt.Println()
		}
		fmt.Println()
	}

	return nil
}

func printCompareSummary(ctx context.Context, client *github.Client, owner, repo, fromRef, toRef string) error {
	// If user passed raw versions, refs may need a leading v. But we also allow SHAs.
	// Try as-is first; if compare fails and refs look like semver, retry with v.

	fromTry := fromRef
	toTry := toRef

	cmp, _, err := client.Repositories.CompareCommits(ctx, owner, repo, fromTry, toTry, &github.ListOptions{PerPage: 100})
	if err != nil {
		fromV := ensureV(fromRef)
		toV := ensureV(toRef)
		if (fromV != fromTry || toV != toTry) && semver.IsValid(fromV) && semver.IsValid(toV) {
			cmp, _, err = client.Repositories.CompareCommits(ctx, owner, repo, fromV, toV, &github.ListOptions{PerPage: 100})
		}
	}
	if err != nil {
		return err
	}

	fmt.Printf("## Compare summary\n\n")
	fmt.Printf("- **Ahead by**: %d commits\n", cmp.GetAheadBy())
	fmt.Printf("- **Behind by**: %d commits\n\n", cmp.GetBehindBy())

	commits := cmp.Commits
	if len(commits) == 0 {
		fmt.Printf("(no commits returned by compare API)\n")
		return nil
	}

	fmt.Printf("### Commits\n\n")
	for _, c := range commits {
		sha := c.GetSHA()
		msg := ""
		if c.Commit != nil {
			msg = strings.Split(strings.TrimSpace(c.Commit.GetMessage()), "\n")[0]
		}
		if len(sha) > 7 {
			sha = sha[:7]
		}
		if msg == "" {
			msg = "(no message)"
		}
		fmt.Printf("- `%s` %s\n", sha, sanitizeInline(msg))
	}
	fmt.Println()

	return nil
}

func sanitizeInline(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	return s
}
