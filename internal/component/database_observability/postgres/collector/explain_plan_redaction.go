package collector

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/grafana/alloy/internal/component/database_observability"
)

// explainPlanExpressionKeys are the keys of an EXPLAIN (FORMAT JSON) plan whose
// string values are deparsed SQL expressions, and can therefore contain
// constants. The list is taken from the labels PostgreSQL emits through
// show_expression, show_sort_group_keys, and friends in
// src/backend/commands/explain.c.
//
// It is not meant to be exhaustive: extensions such as postgres_fdw add their
// own labels (for example "Remote SQL"), and new PostgreSQL versions add more.
// Strings under any other key are handled by redactPlanString.
var explainPlanExpressionKeys = map[string]struct{}{
	"Output":              {},
	"Filter":              {},
	"Index Cond":          {},
	"Order By":            {},
	"Recheck Cond":        {},
	"TID Cond":            {},
	"Join Filter":         {},
	"Merge Cond":          {},
	"Hash Cond":           {},
	"One-Time Filter":     {},
	"Run Condition":       {},
	"Conflict Filter":     {},
	"Function Call":       {},
	"Table Function Call": {},
	"Sort Key":            {},
	"Group Key":           {},
	"Group Keys":          {},
	"Hash Key":            {},
	"Hash Keys":           {},
	"Presorted Key":       {},
	"Cache Key":           {},
	"Sampling Parameters": {},
	"Repeatable Seed":     {},
	"Query Text":          {},
	"Query Parameters":    {},
}

// explainPlanIdentifierKeys are the keys whose values are names that PostgreSQL
// writes into the plan as they are stored, not deparsed expressions. They are
// kept as they are so that names which don't look like plain words, such as
// "2024_orders" or "InitPlan 1 (returns $0)", aren't changed by the SQL
// obfuscator.
//
// A name is not a literal, but it is not always PostgreSQL's own text either:
// "Alias" is whatever the query author wrote, and relation, schema, index, and
// function names are chosen by whoever designed the schema. The structured
// explain plan output already carries the alias unredacted.
var explainPlanIdentifierKeys = map[string]struct{}{
	"Relation Name":            {},
	"Schema":                   {},
	"Alias":                    {},
	"Index Name":               {},
	"CTE Name":                 {},
	"Subplan Name":             {},
	"Function Name":            {},
	"Table Function Name":      {},
	"Tuplestore Name":          {},
	"Conflict Arbiter Indexes": {},
}

// explainPlanSettingsKey is the key EXPLAIN (SETTINGS) puts next to "Plan" for the non-default
// settings that affect planning, such as "work_mem": "64kB" or "enable_hashjoin": "off". Those
// are server configuration, not data from the query, so the values are kept as returned. They
// would otherwise be redacted, because values like "64kB" are not plain words. Only the Settings
// next to the plan is trusted: a key of that name inside the plan is redacted like any other.
const explainPlanSettingsKey = "Settings"

// explainPlanKey is the key of the plan tree. Everything below it is plan nodes.
const explainPlanKey = "Plan"

// explainPlanCollectorSettings are the settings the collector sets on its own explain session.
// EXPLAIN (SETTINGS) reports them as non-default, but they say nothing about the database, and
// search_path lists every schema in it, which can be thousands of tenant names, so they are
// dropped from the logged settings.
var explainPlanCollectorSettings = map[string]struct{}{
	"search_path":              {},
	"plan_cache_mode":          {},
	"enable_partition_pruning": {},
}

// plainWordsPattern matches strings made only of letters, underscores, and
// spaces, such as "Seq Scan" or "orders". Such a string cannot carry a literal:
// a string literal needs a quote and a numeric literal needs a digit, so a
// value like "LIMIT 5" does not match and is redacted. Digits are not allowed
// anywhere for that reason.
var plainWordsPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z_ ]*$`)

// redactExplainPlanJSON returns the EXPLAIN (FORMAT JSON) plan with the SQL
// expressions in it redacted and everything else left as PostgreSQL returned it:
// costs, row estimates, node types, names, and the order of keys and array
// elements are unchanged.
//
// The plan is rewritten token by token, so numbers keep their original text.
func redactExplainPlanJSON(plan []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(plan))
	dec.UseNumber()

	r := &planRedactor{dec: dec}
	r.enc = json.NewEncoder(&r.str)
	r.enc.SetEscapeHTML(false)
	if err := r.value(""); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after explain plan json")
	}
	return r.buf.Bytes(), nil
}

type planRedactor struct {
	dec *json.Decoder
	buf bytes.Buffer

	// str and enc encode one string at a time. They are reused for the whole plan.
	str bytes.Buffer
	enc *json.Encoder

	// inSettings is true while the value of the "Settings" key next to the plan is being copied.
	inSettings bool

	// inPlan is above zero while the value of the "Plan" key is being copied.
	inPlan int
}

// value copies the next JSON value to the output. key is the object key the
// value belongs to. Array elements inherit the key of their array.
func (r *planRedactor) value(key string) error {
	tok, err := r.dec.Token()
	if err != nil {
		return err
	}

	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return r.object()
		case '[':
			return r.array(key)
		default:
			return fmt.Errorf("unexpected delimiter %q in explain plan json", rune(t))
		}
	case string:
		if r.inSettings {
			return r.writeString(t)
		}
		return r.writeString(redactPlanString(key, t))
	case json.Number:
		r.buf.WriteString(t.String())
	case bool:
		r.buf.WriteString(strconv.FormatBool(t))
	case nil:
		r.buf.WriteString("null")
	default:
		return fmt.Errorf("unexpected token type %T in explain plan json", tok)
	}
	return nil
}

func (r *planRedactor) object() error {
	r.buf.WriteByte('{')
	wrote := false
	for r.dec.More() {
		tok, err := r.dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("unexpected object key of type %T in explain plan json", tok)
		}

		// The settings the collector sets itself are left out of the logged settings.
		if r.inSettings {
			if _, own := explainPlanCollectorSettings[key]; own {
				if err := r.skipValue(); err != nil {
					return err
				}
				continue
			}
		}

		if wrote {
			r.buf.WriteByte(',')
		}
		wrote = true

		// Keys are labels chosen by PostgreSQL, not query data.
		if err := r.writeString(key); err != nil {
			return err
		}
		r.buf.WriteByte(':')

		// Only the call that turns a flag on turns it off, so everything inside the object is
		// handled alike.
		enterSettings := key == explainPlanSettingsKey && !r.inSettings && r.inPlan == 0
		enterPlan := key == explainPlanKey
		if enterSettings {
			r.inSettings = true
		}
		if enterPlan {
			r.inPlan++
		}
		err = r.value(key)
		if enterSettings {
			r.inSettings = false
		}
		if enterPlan {
			r.inPlan--
		}
		if err != nil {
			return err
		}
	}
	if _, err := r.dec.Token(); err != nil { // the closing '}'
		return err
	}
	r.buf.WriteByte('}')
	return nil
}

func (r *planRedactor) array(key string) error {
	r.buf.WriteByte('[')
	for first := true; r.dec.More(); first = false {
		if !first {
			r.buf.WriteByte(',')
		}
		if err := r.value(key); err != nil {
			return err
		}
	}
	if _, err := r.dec.Token(); err != nil { // the closing ']'
		return err
	}
	r.buf.WriteByte(']')
	return nil
}

// skipValue reads the next JSON value, whatever it is, and writes nothing.
func (r *planRedactor) skipValue() error {
	tok, err := r.dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || (d != '{' && d != '[') {
		return nil
	}

	for depth := 1; depth > 0; {
		tok, err := r.dec.Token()
		if err != nil {
			return err
		}
		if d, ok := tok.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

// writeString writes s as a JSON string without escaping <, >, and &, so that
// expressions stay readable in the log.
func (r *planRedactor) writeString(s string) error {
	r.str.Reset()
	if err := r.enc.Encode(s); err != nil {
		return err
	}
	r.buf.Write(bytes.TrimRight(r.str.Bytes(), "\n"))
	return nil
}

// redactPlanString redacts a string value found under key. Strings under a
// known expression key are always redacted, and strings under a known
// identifier key are kept. Any other string is kept only when it is plain
// words, so labels this package doesn't know about, such as "Remote SQL", are
// redacted instead of leaked.
func redactPlanString(key, value string) string {
	if _, ok := explainPlanExpressionKeys[key]; ok {
		return database_observability.RedactSql(value)
	}
	if _, ok := explainPlanIdentifierKeys[key]; ok {
		return value
	}
	if plainWordsPattern.MatchString(value) {
		return value
	}
	return database_observability.RedactSql(value)
}
