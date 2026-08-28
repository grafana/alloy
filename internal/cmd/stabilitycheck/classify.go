package main

import (
	"fmt"
	"sort"
	"time"
)

// gaStability is the stability string of a generally-available component. GA
// components are not tracked.
const gaStability = "generally-available"

// registeredComponent is the view of a registered component the checker needs.
// It decouples classification from the component registry so the logic is
// testable without importing every component.
type registeredComponent struct {
	Name      string
	Stability string // "experimental", "public-preview", or "generally-available"
	Community bool
}

// findingKind names why a component or entry failed the check.
type findingKind int

const (
	// missingEntry: a non-GA component has no entry.
	missingEntry findingKind = iota
	// levelMismatch: the entry stability disagrees with the code.
	levelMismatch
	// expired: the entry review deadline has passed.
	expired
	// staleGA: an entry names a component that is now GA.
	staleGA
	// staleMissing: an entry names a component that no longer exists.
	staleMissing
	// staleCommunity: an entry names a community component, which is not tracked.
	staleCommunity
)

// finding is one failure to report.
type finding struct {
	kind    findingKind
	name    string
	message string
}

// classify compares the registered components against the config and returns
// every failure. It never mutates its inputs. Results are sorted by name so
// output is stable.
func classify(comps []registeredComponent, cfg *Config, now time.Time) []finding {
	byName := make(map[string]registeredComponent, len(comps))
	for _, c := range comps {
		byName[c.Name] = c
	}
	entries := cfg.entryByName()

	var findings []finding

	// Every non-GA component must have a valid, current, level-matching entry.
	for _, c := range comps {
		if c.Community || c.Stability == gaStability {
			continue
		}
		e, ok := entries[c.Name]
		if !ok {
			findings = append(findings, finding{
				kind:    missingEntry,
				name:    c.Name,
				message: missingEntrySnippet(c),
			})
			continue
		}
		if e.Stability != c.Stability {
			findings = append(findings, finding{
				kind:    levelMismatch,
				name:    c.Name,
				message: fmt.Sprintf("entry says stability %q but code says %q — update and re-justify", e.Stability, c.Stability),
			})
		}
		if now.After(e.Expires) {
			findings = append(findings, finding{
				kind:    expired,
				name:    c.Name,
				message: fmt.Sprintf("review expired on %s — renew or remove the entry", e.Expires.Format("2006-01-02")),
			})
		}
	}

	// Every entry must name a currently non-GA, non-community component.
	for _, e := range cfg.Components {
		c, ok := byName[e.Name]
		switch {
		case !ok:
			findings = append(findings, finding{
				kind:    staleMissing,
				name:    e.Name,
				message: "component no longer exists — remove the entry",
			})
		case c.Community:
			findings = append(findings, finding{
				kind:    staleCommunity,
				name:    e.Name,
				message: "component is a community component and must not be tracked — remove the entry",
			})
		case c.Stability == gaStability:
			findings = append(findings, finding{
				kind:    staleGA,
				name:    e.Name,
				message: "component is now generally available — remove the entry",
			})
		}
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].name != findings[j].name {
			return findings[i].name < findings[j].name
		}
		return findings[i].kind < findings[j].kind
	})
	return findings
}

// missingEntrySnippet returns a paste-ready YAML block for a component that has
// no entry. The reason and expires fields are left for a human.
func missingEntrySnippet(c registeredComponent) string {
	return fmt.Sprintf("no entry in .stability.yaml — add (keep entries sorted by name):\n"+
		"      - name: %s\n"+
		"        stability: %s\n"+
		"        reason: <why this is not yet generally available>\n"+
		"        expires: <YYYY-MM-DD>",
		c.Name, c.Stability)
}
