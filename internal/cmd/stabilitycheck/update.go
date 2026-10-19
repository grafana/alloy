package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// updateResult reports what runUpdate changed.
type updateResult struct {
	added []string // names of component entries added
	stale []string // existing component entries that are no longer non-GA
}

// runUpdate rewrites path in place: it adds a TODO entry for every non-GA
// component that has no entry, sorts both sections by name, and wraps long
// reasons. Existing entries keep their values. Stale entries are reported, not removed, because removing
// an entry would discard its human-written reason. It works on a yaml.Node tree
// so the file header and comments survive.
func runUpdate(path string, comps []registeredComponent, defaultExpires time.Time, w io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	var doc yaml.Node
	if len(bytes.TrimSpace(data)) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse YAML: %w", err)
		}
	}
	top := topMapping(&doc)

	compSeq := findOrCreateSeq(top, "components")
	featSeq := findSeq(top, "features") // features are sorted only, never added.

	nonGA := make(map[string]string, len(comps)) // name -> stability
	for _, c := range comps {
		if c.Community || c.Stability == gaStability {
			continue
		}
		nonGA[c.Name] = c.Stability
	}

	existing := namesInSeq(compSeq)

	var res updateResult
	for name, stability := range nonGA {
		if existing[name] {
			continue
		}
		compSeq.Content = append(compSeq.Content, newEntryNode(name, stability, placeholderReason, defaultExpires))
		res.added = append(res.added, name)
	}
	// An existing entry whose component is not currently non-GA is stale.
	for name := range existing {
		if _, ok := nonGA[name]; !ok {
			res.stale = append(res.stale, name)
		}
	}

	sortSeqByName(compSeq)
	wrapReasons(compSeq)
	if featSeq != nil {
		sortSeqByName(featSeq)
		wrapReasons(featSeq)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2) // Match the existing 2-space style.
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("encode YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("close encoder: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return err
	}

	res.report(w, path)
	return nil
}

func (r updateResult) report(w io.Writer, path string) {
	sort.Strings(r.added)
	sort.Strings(r.stale)
	_, _ = fmt.Fprintf(w, "stabilitycheck --update: wrote %s\n", path)
	_, _ = fmt.Fprintf(w, "  added %d component entr(y/ies), sorted and wrapped all sections\n", len(r.added))
	for _, n := range r.added {
		_, _ = fmt.Fprintf(w, "    + %s (reason left as %s)\n", n, placeholderReason)
	}
	if len(r.stale) > 0 {
		_, _ = fmt.Fprintf(w, "  %d stale entr(y/ies) left in place — remove by hand (they keep their reason):\n", len(r.stale))
		for _, n := range r.stale {
			_, _ = fmt.Fprintf(w, "    ! %s\n", n)
		}
	}
}

// topMapping returns the top-level mapping node, creating the document and
// mapping if the file was empty.
func topMapping(doc *yaml.Node) *yaml.Node {
	if len(doc.Content) == 0 {
		m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{m}
		return m
	}
	return doc.Content[0]
}

// findSeq returns the sequence node for key, or nil if the key is absent.
func findSeq(top *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == key {
			return top.Content[i+1]
		}
	}
	return nil
}

// findOrCreateSeq returns the sequence node for key, appending an empty one if
// the key is absent.
func findOrCreateSeq(top *yaml.Node, key string) *yaml.Node {
	if seq := findSeq(top, key); seq != nil {
		return seq
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	top.Content = append(top.Content, keyNode, seq)
	return seq
}

// namesInSeq returns the set of entry names in a sequence node.
func namesInSeq(seq *yaml.Node) map[string]bool {
	out := make(map[string]bool, len(seq.Content))
	for _, e := range seq.Content {
		if n := entryName(e); n != "" {
			out[n] = true
		}
	}
	return out
}

// entryName reads the `name` value from an entry mapping node.
func entryName(m *yaml.Node) string {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == "name" {
			return m.Content[i+1].Value
		}
	}
	return ""
}

// sortSeqByName sorts a sequence node's entries alphabetically by name.
func sortSeqByName(seq *yaml.Node) {
	sort.SliceStable(seq.Content, func(i, j int) bool {
		return entryName(seq.Content[i]) < entryName(seq.Content[j])
	})
}

// newEntryNode builds an entry mapping node with a TODO reason.
func newEntryNode(name, stability, reason string, expires time.Time) *yaml.Node {
	str := func(v string) *yaml.Node { return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v} }
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = []*yaml.Node{
		str("name"), str(name),
		str("stability"), str(stability),
		str("reason"), str(reason),
		str("expires"), {Kind: yaml.ScalarNode, Tag: "!!timestamp", Value: expires.Format("2006-01-02")},
	}
	return m
}

// reasonWidth is the longest reason line. Entry keys are at column 4, so a
// reason block line starts at column 6 and ends at or before column 80.
const reasonWidth = 74

// wrapReasons rewrites each reason that has a line longer than reasonWidth as a
// literal block string. The YAML encoder cannot wrap lines, and a folded block
// string does not survive a round trip, so the line breaks go into the value.
// The reason is only read by people, so the extra line breaks are harmless.
// A reason that already fits is left unchanged, so hand-made line breaks stay.
func wrapReasons(seq *yaml.Node) {
	for _, e := range seq.Content {
		for i := 0; i+1 < len(e.Content); i += 2 {
			if e.Content[i].Value != "reason" {
				continue
			}
			v := e.Content[i+1]
			if !hasLongLine(v.Value, reasonWidth) {
				continue
			}
			v.Value = wrapText(v.Value, reasonWidth)
			v.Style = yaml.LiteralStyle
		}
	}
}

func hasLongLine(s string, width int) bool {
	for line := range strings.SplitSeq(s, "\n") {
		if len(line) > width {
			return true
		}
	}
	return false
}

// wrapText joins the words of s into lines of at most width bytes. A word
// longer than width gets a line of its own.
func wrapText(s string, width int) string {
	var b strings.Builder
	lineLen := 0
	for _, word := range strings.Fields(s) {
		switch {
		case lineLen == 0:
		case lineLen+1+len(word) > width:
			b.WriteByte('\n')
			lineLen = 0
		default:
			b.WriteByte(' ')
			lineLen++
		}
		b.WriteString(word)
		lineLen += len(word)
	}
	return b.String()
}
