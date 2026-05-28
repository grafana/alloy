package main

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// report accumulates per-module scan results and decides what's actionable
// vs. ignored after applying the config.
type report struct {
	cfg *Config
	now time.Time

	// Per-module bookkeeping, in scan order.
	modules []moduleReport
}

type moduleReport struct {
	Module     string
	Actionable []vulnerability
	Ignored    []ignoredFinding
}

type ignoredFinding struct {
	Vuln  vulnerability
	Entry IgnoreEntry
}

func newReport(cfg *Config, now time.Time) *report {
	return &report{cfg: cfg, now: now}
}

func (r *report) add(module string, result *scanResult) {
	mr := moduleReport{Module: module}
	for _, v := range result.Vulns {
		if entry := r.cfg.isIgnored(v.ID, r.now); entry != nil {
			mr.Ignored = append(mr.Ignored, ignoredFinding{Vuln: v, Entry: *entry})
			continue
		}
		mr.Actionable = append(mr.Actionable, v)
	}
	r.modules = append(r.modules, mr)
}

func (r *report) hasActionable() bool {
	for _, m := range r.modules {
		if len(m.Actionable) > 0 {
			return true
		}
	}
	return false
}

// print writes a human-readable report. Per-module sections list actionable
// findings with one example call trace each; ignored findings are listed as
// a one-liner with their reason. Reasoning behind the formatting choices:
// CI logs are scanned with eyes, so the actionable items go in full detail
// and ignored items get summarized to keep the noise down.
func (r *report) print(w io.Writer) {
	var totalActionable, totalIgnored int
	for _, m := range r.modules {
		totalActionable += len(m.Actionable)
		totalIgnored += len(m.Ignored)
		fmt.Fprintf(w, "\n==> %s\n", m.Module)
		if len(m.Actionable) == 0 && len(m.Ignored) == 0 {
			fmt.Fprintln(w, "    no reachable vulnerabilities")
			continue
		}
		for _, v := range m.Actionable {
			printActionable(w, v)
		}
		for _, ig := range m.Ignored {
			printIgnored(w, ig)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d actionable, %d ignored (across %d modules).\n",
		totalActionable, totalIgnored, len(r.modules))
	if totalActionable == 0 {
		fmt.Fprintln(w, "OK — no actionable reachable vulnerabilities.")
	}
}

func printActionable(w io.Writer, v vulnerability) {
	fmt.Fprintf(w, "    [FAIL] %s — %s\n", v.ID, v.Summary)
	fmt.Fprintf(w, "           module:  %s\n", v.Module)
	if v.FixedVersion != "" {
		fmt.Fprintf(w, "           fix:     upgrade to %s\n", v.FixedVersion)
	} else {
		fmt.Fprintf(w, "           fix:     none available upstream\n")
	}
	fmt.Fprintf(w, "           details: https://pkg.go.dev/vuln/%s\n", v.ID)
	if trace := renderTrace(v.ExampleTrace); trace != "" {
		fmt.Fprintf(w, "           trace:   %s\n", trace)
	}
}

func printIgnored(w io.Writer, ig ignoredFinding) {
	fmt.Fprintf(w, "    [IGN]  %s — %s\n", ig.Vuln.ID, ig.Vuln.Summary)
	fmt.Fprintf(w, "           reason:  %s\n", ig.Entry.Reason)
	if !ig.Entry.Expires.IsZero() {
		fmt.Fprintf(w, "           expires: %s\n", ig.Entry.Expires.Format("2006-01-02"))
	}
}

// renderTrace renders one example call chain entry-point → vulnerable symbol.
// govulncheck emits frames vulnerable-symbol-first, so we reverse the order
// when displaying so it reads top-down like a stack trace.
func renderTrace(trace []frame) string {
	if len(trace) == 0 {
		return ""
	}
	// Reverse: entry point first → vuln last.
	ordered := make([]frame, len(trace))
	for i, f := range trace {
		ordered[len(trace)-1-i] = f
	}

	parts := make([]string, 0, len(ordered))
	for _, f := range ordered {
		label := f.Function
		if label == "" {
			if f.Package != "" {
				label = f.Package
			} else {
				label = f.Module
			}
		}
		if f.Position != nil && f.Position.Filename != "" && f.Position.Line > 0 {
			label = fmt.Sprintf("%s (%s:%d)", label, f.Position.Filename, f.Position.Line)
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, " → ")
}
