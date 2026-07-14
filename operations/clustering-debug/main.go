// Command clustering-debug fetches clustering + scrape-target info from a single
// Alloy instance and prints it as one JSON document.
//
// Given the network location of one Alloy instance (for example a pod you have
// port-forwarded out of Kubernetes), it queries Alloy's internal HTTP API and
// reports:
//
//   - the cluster peers this instance knows about (and which peer is "self"),
//   - every running component and its health,
//   - for each scrape-like component, the targets this instance is actively
//     scraping, including per-target health, last scrape time, scrape duration
//     and last error.
//
// Each instance only reports the targets it actively scrapes (after clustering
// distribution, Alloy feeds each prometheus.scrape only its locally-owned
// targets), so the union across all pods is the full fleet view. Run this
// against every pod and correlate the results to spot overlaps, gaps, slow
// endpoints and imbalance.
//
// It uses only data Alloy already exposes — no changes to Alloy or Prometheus,
// no extra metrics, no tracing. The relevant endpoints are:
//
//	GET {prefix}/peers            -> cluster peers (internal/web/api/api.go)
//	GET {prefix}/components       -> component list + health
//	GET {prefix}/components/{id}  -> per-component debugInfo (scrape TargetStatus)
//
// debugInfo is encoded in Alloy-JSON (see syntax/encoding/alloyjson): a body is
// a list of statements; a statement is a block ({name,type:"block",body}) or an
// attribute ({name,type:"attr",value}); a value is {type,value}.
package main

import (
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// defaultAPIPrefix is where Alloy mounts its internal UI/API (see
// internal/service/ui/ui.go and internal/service/http/supportbundle.go).
const defaultAPIPrefix = "/api/v0/web"

// fallbackAPIPrefix is an alternate mount point tried if the default 404s.
const fallbackAPIPrefix = "/api/v0/component"

// stmt is an Alloy-JSON statement: either a block (with Body) or an attribute
// (with Value). Mirrors syntax/encoding/alloyjson.
type stmt struct {
	Name  string  `json:"name"`
	Type  string  `json:"type"`
	Label string  `json:"label,omitempty"`
	Body  []stmt  `json:"body,omitempty"`
	Value *jvalue `json:"value,omitempty"`
}

// jvalue is a single Alloy-JSON value.
type jvalue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

// simplifyValue collapses an Alloy-JSON value into a plain Go value.
func simplifyValue(v *jvalue) any {
	if v == nil {
		return nil
	}
	switch v.Type {
	case "null":
		return nil
	case "string", "number", "bool", "capsule", "function":
		var out any
		_ = json.Unmarshal(v.Value, &out)
		return out
	case "array":
		var elems []jvalue
		if err := json.Unmarshal(v.Value, &elems); err != nil {
			return nil
		}
		out := make([]any, 0, len(elems))
		for i := range elems {
			out = append(out, simplifyValue(&elems[i]))
		}
		return out
	case "object":
		var fields []struct {
			Key   string `json:"key"`
			Value jvalue `json:"value"`
		}
		if err := json.Unmarshal(v.Value, &fields); err != nil {
			return nil
		}
		out := make(map[string]any, len(fields))
		for i := range fields {
			out[fields[i].Key] = simplifyValue(&fields[i].Value)
		}
		return out
	default:
		var out any
		_ = json.Unmarshal(v.Value, &out)
		return out
	}
}

// extractBlocks recursively collects Alloy-JSON blocks with the given name,
// flattening each block's attributes into a map of attr name -> value.
func extractBlocks(body []stmt, name string) []map[string]any {
	var found []map[string]any
	for i := range body {
		s := &body[i]
		if s.Type != "block" {
			continue
		}
		if s.Name == name {
			attrs := make(map[string]any, len(s.Body))
			for j := range s.Body {
				inner := &s.Body[j]
				if inner.Type == "attr" {
					attrs[inner.Name] = simplifyValue(inner.Value)
				}
			}
			found = append(found, attrs)
		} else {
			found = append(found, extractBlocks(s.Body, name)...)
		}
	}
	return found
}

// headerFlags collects repeated -header values.
type headerFlags []string

func (h *headerFlags) String() string { return strings.Join(*h, ", ") }
func (h *headerFlags) Set(v string) error {
	if !strings.Contains(v, ":") {
		return fmt.Errorf("invalid header %q; expected 'Key: Value'", v)
	}
	*h = append(*h, v)
	return nil
}

type client struct {
	base    string
	prefix  string
	headers map[string]string
	http    *http.Client
}

func (c *client) get(path string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, c.base+c.prefix+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// compInfo is a component list entry (only the fields we use).
type compInfo struct {
	Name     string          `json:"name"`
	LocalID  string          `json:"localID"`
	ModuleID string          `json:"moduleID"`
	Label    string          `json:"label"`
	Health   json.RawMessage `json:"health"`
}

func (ci compInfo) id() string {
	if ci.ModuleID != "" {
		return ci.ModuleID + "/" + ci.LocalID
	}
	return ci.LocalID
}

// compDetail is a single-component response (only the fields we use).
type compDetail struct {
	Health    json.RawMessage `json:"health"`
	DebugInfo []stmt          `json:"debugInfo"`
}

type compSummary struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	ModuleID string          `json:"module_id"`
	Label    string          `json:"label"`
	Health   json.RawMessage `json:"health"`
}

type scrapeComp struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Label       string           `json:"label"`
	Health      json.RawMessage  `json:"health"`
	TargetCount int              `json:"target_count"`
	Targets     []map[string]any `json:"targets"`
}

type fetchErr struct {
	Endpoint string `json:"endpoint"`
	Error    string `json:"error"`
}

type result struct {
	Pod              string          `json:"pod,omitempty"`
	Source           string          `json:"source"`
	APIPrefix        string          `json:"api_prefix"`
	FetchedAt        string          `json:"fetched_at"`
	Self             any             `json:"self"`
	Peers            json.RawMessage `json:"peers"`
	Components       []compSummary   `json:"components"`
	ScrapeComponents []scrapeComp    `json:"scrape_components"`
	Errors           []fetchErr      `json:"errors"`
}

func main() {
	var (
		output      = flag.String("output", "", "single-instance mode: write JSON to this file instead of stdout")
		filter      = flag.String("filter", "scrape", "regex of component names to inspect for targets")
		all         = flag.Bool("all", false, "fetch debug info for every component, not just matching ones")
		timeout     = flag.Duration("timeout", 10*time.Second, "per-request timeout")
		insecure    = flag.Bool("insecure", false, "skip TLS certificate verification")
		prefix      = flag.String("api-prefix", defaultAPIPrefix, "API path prefix (falls back automatically if the default 404s)")
		statefulset = flag.String("statefulset", "", "StatefulSet mode: collect from every pod owned by this StatefulSet (via kubectl port-forward)")
		namespace   = flag.String("namespace", "", "Kubernetes namespace for -statefulset (defaults to current context namespace)")
		remotePort  = flag.Int("remote-port", 0, "StatefulSet mode: the pod port to port-forward to (the Alloy HTTP port)")
		outputDir   = flag.String("output-dir", ".", "StatefulSet mode: directory to write <pod>.json files into")
		headers     headerFlags
	)
	flag.StringVar(namespace, "n", "", "alias for -namespace")
	flag.Var(&headers, "header", "extra HTTP header 'Key: Value' (repeatable)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage:
  %[1]s [flags] BASE_URL
        Collect from one instance. BASE_URL is the Alloy base URL,
        e.g. http://localhost:12345 (it may appear before or after flags).

  %[1]s -statefulset NAME -remote-port PORT [flags]
        Collect from every pod of a StatefulSet by port-forwarding each one
        with kubectl, writing <pod>.json into -output-dir. Requires kubectl.

Flags:
`, os.Args[0])
		flag.PrintDefaults()
	}

	// The standard flag package stops parsing at the first positional argument,
	// so pull the base URL out of the args first. This lets the URL appear
	// before or after flags (e.g. "BASE -output x" as well as "-output x BASE").
	var base string
	var rest []string
	for _, a := range os.Args[1:] {
		if base == "" && (strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://")) {
			base = a
			continue
		}
		rest = append(rest, a)
	}
	_ = flag.CommandLine.Parse(rest)

	compRe, err := regexp.Compile(*filter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -filter regex: %v\n", err)
		os.Exit(2)
	}

	hdr := map[string]string{}
	for _, h := range headers {
		k, v, _ := strings.Cut(h, ":")
		hdr[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	transport := &http.Transport{}
	if *insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	httpClient := &http.Client{Timeout: *timeout, Transport: transport}

	opts := collectOptions{prefix: *prefix, headers: hdr, http: httpClient, filter: compRe, all: *all}

	// StatefulSet mode: collect from every pod via kubectl port-forward.
	if *statefulset != "" {
		if *remotePort == 0 {
			fmt.Fprintln(os.Stderr, "error: -statefulset requires -remote-port")
			flag.Usage()
			os.Exit(2)
		}
		os.Exit(runStatefulSet(*namespace, *statefulset, *remotePort, *outputDir, opts))
	}

	// Single-instance mode.
	if base == "" {
		fmt.Fprintln(os.Stderr, "error: missing BASE_URL (must start with http:// or https://), or use -statefulset")
		flag.Usage()
		os.Exit(2)
	}
	res := collect(strings.TrimRight(base, "/"), opts)
	if err := emit(res, *output); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if *output != "" {
		fmt.Fprintf(os.Stderr, "wrote %s\n", *output)
	}
}

// collectOptions bundles everything collect needs besides the base URL.
type collectOptions struct {
	prefix  string
	headers map[string]string
	http    *http.Client
	filter  *regexp.Regexp
	all     bool
}

// collect gathers peers + components + scrape-target detail from one instance.
func collect(base string, opts collectOptions) result {
	c := &client{base: base, prefix: opts.prefix, headers: opts.headers, http: opts.http}
	c.prefix = detectPrefix(c, opts.prefix)

	res := result{
		Source:           base,
		APIPrefix:        c.prefix,
		FetchedAt:        time.Now().UTC().Format(time.RFC3339),
		Peers:            json.RawMessage("[]"),
		Components:       []compSummary{},
		ScrapeComponents: []scrapeComp{},
		Errors:           []fetchErr{},
	}

	// 1. Cluster peers (fails cleanly if clustering is disabled).
	if peersRaw, err := c.get("/peers"); err != nil {
		res.Errors = append(res.Errors, fetchErr{"/peers", err.Error()})
	} else {
		res.Peers = peersRaw
		res.Self = findSelf(peersRaw)
	}

	// 2. Component list (health only).
	componentsRaw, err := c.get("/components")
	if err != nil {
		res.Errors = append(res.Errors, fetchErr{"/components", err.Error()})
		return res
	}
	var components []compInfo
	if err := json.Unmarshal(componentsRaw, &components); err != nil {
		res.Errors = append(res.Errors, fetchErr{"/components", "decode: " + err.Error()})
		return res
	}

	for _, comp := range components {
		id := comp.id()
		res.Components = append(res.Components, compSummary{
			ID: id, Name: comp.Name, ModuleID: comp.ModuleID, Label: comp.Label, Health: comp.Health,
		})

		if !opts.all && !opts.filter.MatchString(comp.Name) {
			continue
		}

		// 3. Per-component debug info -> target blocks.
		detailRaw, err := c.get("/components/" + url.PathEscape(id))
		if err != nil {
			res.Errors = append(res.Errors, fetchErr{"/components/" + id, err.Error()})
			continue
		}
		var detail compDetail
		if err := json.Unmarshal(detailRaw, &detail); err != nil {
			res.Errors = append(res.Errors, fetchErr{"/components/" + id, "decode: " + err.Error()})
			continue
		}

		targets := extractBlocks(detail.DebugInfo, "target")
		if len(targets) == 0 && !opts.all {
			continue // matched the filter but exposes no targets
		}
		health := detail.Health
		if len(health) == 0 {
			health = comp.Health
		}
		res.ScrapeComponents = append(res.ScrapeComponents, scrapeComp{
			ID: id, Name: comp.Name, Label: comp.Label, Health: health,
			TargetCount: len(targets), Targets: targets,
		})
	}

	return res
}

// runStatefulSet port-forwards to each pod of the StatefulSet in turn, collects
// from it, and writes <pod>.json into outputDir. Returns a process exit code.
func runStatefulSet(namespace, statefulset string, remotePort int, outputDir string, opts collectOptions) int {
	pods, err := statefulsetPods(namespace, statefulset)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	if len(pods) == 0 {
		fmt.Fprintf(os.Stderr, "error: no pods found for StatefulSet %q\n", statefulset)
		return 1
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stderr, "found %d pod(s) for StatefulSet %q: %s\n", len(pods), statefulset, strings.Join(pods, ", "))
	failures := 0
	for _, pod := range pods {
		fmt.Fprintf(os.Stderr, "==> %s: port-forwarding to :%d ...\n", pod, remotePort)
		localPort, stop, err := startPortForward(namespace, pod, remotePort, 15*time.Second)
		if err != nil {
			fmt.Fprintf(os.Stderr, "    skip %s: %v\n", pod, err)
			failures++
			continue
		}

		res := collect(fmt.Sprintf("http://127.0.0.1:%d", localPort), opts)
		res.Pod = pod
		stop()

		path := filepath.Join(outputDir, pod+".json")
		if err := emit(res, path); err != nil {
			fmt.Fprintf(os.Stderr, "    failed to write %s: %v\n", path, err)
			failures++
			continue
		}
		fmt.Fprintf(os.Stderr, "    wrote %s (%d scrape components, %d targets total)\n",
			path, len(res.ScrapeComponents), totalTargets(res))
	}
	if failures > 0 {
		fmt.Fprintf(os.Stderr, "completed with %d failure(s)\n", failures)
		return 1
	}
	return 0
}

func totalTargets(res result) int {
	n := 0
	for _, sc := range res.ScrapeComponents {
		n += sc.TargetCount
	}
	return n
}

// kubectlArgs prepends namespace selection (if any) to a kubectl invocation.
func kubectlArgs(namespace string, args ...string) []string {
	if namespace != "" {
		return append([]string{"-n", namespace}, args...)
	}
	return args
}

// runKubectl runs kubectl and returns stdout, surfacing stderr on failure.
func runKubectl(args ...string) ([]byte, error) {
	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("kubectl %s: %s", strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("kubectl %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// statefulsetPods returns the names of pods owned by the given StatefulSet,
// sorted by ordinal. It filters on ownerReferences so it only matches pods that
// actually belong to the StatefulSet.
func statefulsetPods(namespace, statefulset string) ([]string, error) {
	out, err := runKubectl(kubectlArgs(namespace, "get", "pods", "-o", "json")...)
	if err != nil {
		return nil, err
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name            string `json:"name"`
				OwnerReferences []struct {
					Kind string `json:"kind"`
					Name string `json:"name"`
				} `json:"ownerReferences"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("decode pod list: %w", err)
	}
	var names []string
	for _, it := range list.Items {
		for _, o := range it.Metadata.OwnerReferences {
			if o.Kind == "StatefulSet" && o.Name == statefulset {
				names = append(names, it.Metadata.Name)
				break
			}
		}
	}
	sortByOrdinal(names)
	return names, nil
}

// sortByOrdinal sorts StatefulSet pod names (e.g. alloy-0, alloy-1, alloy-10) by
// their trailing ordinal rather than lexically.
func sortByOrdinal(names []string) {
	ordinal := func(s string) int {
		i := strings.LastIndex(s, "-")
		if i < 0 {
			return -1
		}
		n, err := strconv.Atoi(s[i+1:])
		if err != nil {
			return -1
		}
		return n
	}
	sort.SliceStable(names, func(i, j int) bool { return ordinal(names[i]) < ordinal(names[j]) })
}

var forwardingRe = regexp.MustCompile(`Forwarding from .*?:(\d+) ->`)

// startPortForward runs `kubectl port-forward pod/<pod> :<remotePort>`, letting
// kubectl pick a free local port. It returns the chosen local port and a stop
// function to tear the forward down once collection is complete.
func startPortForward(namespace, pod string, remotePort int, timeout time.Duration) (int, func(), error) {
	args := kubectlArgs(namespace, "port-forward", "pod/"+pod, fmt.Sprintf(":%d", remotePort))
	cmd := exec.Command("kubectl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return 0, nil, err
	}
	stop := func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}

	portCh := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if m := forwardingRe.FindStringSubmatch(sc.Text()); m != nil {
				p, _ := strconv.Atoi(m[1])
				portCh <- p
				return
			}
		}
		errCh <- fmt.Errorf("port-forward exited: %s", strings.TrimSpace(stderr.String()))
	}()

	select {
	case p := <-portCh:
		return p, stop, nil
	case err := <-errCh:
		stop()
		return 0, nil, err
	case <-time.After(timeout):
		stop()
		return 0, nil, fmt.Errorf("timed out waiting for port-forward (kubectl: %s)", strings.TrimSpace(stderr.String()))
	}
}

// detectPrefix returns a working API prefix, falling back if the requested one
// 404s (or otherwise fails) and a fallback exists.
func detectPrefix(c *client, requested string) string {
	candidates := []string{requested}
	if requested == defaultAPIPrefix {
		candidates = append(candidates, fallbackAPIPrefix)
	}
	for _, p := range candidates {
		c.prefix = p
		if _, err := c.get("/components"); err == nil {
			return p
		}
	}
	return requested
}

// findSelf returns the peer object whose Self/self field is true, or nil.
func findSelf(peersRaw []byte) any {
	var peers []map[string]any
	if err := json.Unmarshal(peersRaw, &peers); err != nil {
		return nil
	}
	for _, p := range peers {
		for k, v := range p {
			if strings.EqualFold(k, "self") {
				if b, ok := v.(bool); ok && b {
					return p
				}
			}
		}
	}
	return nil
}

// emit writes res as indented JSON to output (a file path), or to stdout when
// output is empty. It returns an error so callers can decide how to handle it;
// the single-instance path treats a failure as fatal.
func emit(res result, output string) error {
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	if output != "" {
		if err := os.WriteFile(output, append(b, '\n'), 0o644); err != nil {
			return err
		}
		return nil
	}
	fmt.Println(string(b))
	return nil
}
