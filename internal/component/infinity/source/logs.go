package source

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

type entry struct {
	ts                 time.Time
	line               string
	labels             model.LabelSet
	structuredMetadata push.LabelsAdapter
}

// frameToEntries maps each row to one log entry. There is no dedup across
// polls.
func frameToEntries(f *data.Frame, job, instance string, l logsSpec, now time.Time) ([]entry, error) {
	if l.entryLimit > 0 && f.Rows() > l.entryLimit {
		return nil, newPollError(reasonEntryLimit, fmt.Errorf("query returned more than entry_limit (%d) entries", l.entryLimit))
	}

	timeFields := timeFieldsOf(f)
	lineField := fieldByName(f, l.lineColumn)
	// Each row takes its level from the first of these with a value.
	var levelFields []*data.Field
	for _, name := range []string{"severity", "level"} {
		if fld := fieldByName(f, name); fld != nil {
			levelFields = append(levelFields, fld)
		}
	}

	// Resolve label columns once before the row loop, skipping missing ones.
	type resolvedCol struct {
		name  string
		field *data.Field
	}
	var labelCols []resolvedCol
	for _, c := range l.labelColumns {
		fld := fieldByName(f, c)
		if fld != nil {
			labelCols = append(labelCols, resolvedCol{name: c, field: fld})
		}
	}

	// Resolve structured metadata columns once before the row loop, skipping missing ones.
	var metadataCols []resolvedCol
	for _, c := range l.structuredMetadataColumns {
		fld := fieldByName(f, c)
		if fld != nil {
			metadataCols = append(metadataCols, resolvedCol{name: c, field: fld})
		}
	}

	entries := make([]entry, 0, f.Rows())
	for row := 0; row < f.Rows(); row++ {
		e := entry{ts: now, labels: model.LabelSet{}}
		for _, tf := range timeFields {
			if v, ok := tf.ConcreteAt(row); ok {
				e.ts = v.(time.Time)
				break
			}
		}
		if lineField != nil {
			e.line = valueString(lineField, row)
		} else {
			line, err := rowJSON(f, row)
			if err != nil {
				return nil, newPollError(reasonParse, err)
			}
			e.line = line
		}
		for _, col := range labelCols {
			if _, ok := col.field.ConcreteAt(row); ok {
				e.labels[model.LabelName(sanitizeName(col.name))] = model.LabelValue(valueString(col.field, row))
			}
		}
		if _, set := e.labels["level"]; !set {
			for _, lf := range levelFields {
				if _, ok := lf.ConcreteAt(row); ok {
					e.labels["level"] = model.LabelValue(valueString(lf, row))
					break
				}
			}
		}
		for _, col := range metadataCols {
			if _, ok := col.field.ConcreteAt(row); ok {
				e.structuredMetadata = append(e.structuredMetadata, push.LabelAdapter{Name: col.name, Value: valueString(col.field, row)})
			}
		}
		e.labels[model.JobLabel] = model.LabelValue(job)
		e.labels[model.InstanceLabel] = model.LabelValue(instance)
		entries = append(entries, e)
	}
	return entries, nil
}

// timeFieldsOf returns every time-typed field of f, in column order. A row's
// timestamp is the first of these fields that has a non-nil value in that
// row, so a later row can use a different column than an earlier row does.
func timeFieldsOf(f *data.Frame) []*data.Field {
	var out []*data.Field
	for _, fld := range f.Fields {
		if fld.Type().NonNullableType() == data.FieldTypeTime {
			out = append(out, fld)
		}
	}
	return out
}

func fieldByName(f *data.Frame, name string) *data.Field {
	for _, fld := range f.Fields {
		if fld.Name == name {
			return fld
		}
	}
	return nil
}

func valueString(fld *data.Field, row int) string {
	v, ok := fld.ConcreteAt(row)
	if !ok {
		return ""
	}
	if t, isTime := v.(time.Time); isTime {
		return t.Format(time.RFC3339Nano)
	}
	return fmt.Sprint(v)
}

// rowJSON encodes a row as a JSON object. Keys keep the column order, which
// a Go map cannot do.
func rowJSON(f *data.Frame, row int) (string, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, fld := range f.Fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(fld.Name)
		if err != nil {
			return "", err
		}
		buf.Write(k)
		buf.WriteByte(':')
		var v any
		if cv, ok := fld.ConcreteAt(row); ok {
			v = jsonSafe(cv)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		buf.Write(b)
	}
	buf.WriteByte('}')
	return buf.String(), nil
}

// jsonSafe returns NaN and Inf as the strings "NaN", "+Inf" and "-Inf".
// json.Marshal fails on them, and one such value would fail the whole poll.
func jsonSafe(v any) any {
	var f float64
	switch x := v.(type) {
	case float64:
		f = x
	case float32:
		f = float64(x)
	default:
		return v
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return v
}
