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
// "2024_orders", aren't changed by the SQL obfuscator.
var explainPlanIdentifierKeys = map[string]struct{}{
	"Relation Name": {},
	"Schema":        {},
	"Alias":         {},
	"Index Name":    {},
	"CTE Name":      {},
	"Subplan Name":  {},
}

// plainWordsPattern matches strings made only of words, such as "Seq Scan",
// "orders", or "SubPlan 2". A string that can carry a literal needs a quote, an
// operator, a parenthesis, or a leading digit, none of which match.
var plainWordsPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_ ]*$`)

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
	for first := true; r.dec.More(); first = false {
		if !first {
			r.buf.WriteByte(',')
		}

		tok, err := r.dec.Token()
		if err != nil {
			return err
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("unexpected object key of type %T in explain plan json", tok)
		}

		// Keys are labels chosen by PostgreSQL, not query data.
		if err := r.writeString(key); err != nil {
			return err
		}
		r.buf.WriteByte(':')

		if err := r.value(key); err != nil {
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

// writeString writes s as a JSON string without escaping <, >, and &, so that
// expressions stay readable in the log.
func (r *planRedactor) writeString(s string) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	r.buf.Write(bytes.TrimRight(b.Bytes(), "\n"))
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
