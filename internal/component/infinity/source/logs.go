package source

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	timeField := firstTimeField(f)
	lineField := fieldByName(f, l.lineColumn)
	levelField := fieldByName(f, "severity")
	if levelField == nil {
		levelField = fieldByName(f, "level")
	}

	entries := make([]entry, 0, f.Rows())
	for row := 0; row < f.Rows(); row++ {
		e := entry{ts: now, labels: model.LabelSet{}}
		if timeField != nil {
			if v, ok := timeField.ConcreteAt(row); ok {
				e.ts = v.(time.Time)
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
		for _, c := range l.labelColumns {
			fld := fieldByName(f, c)
			if fld == nil {
				continue
			}
			if _, ok := fld.ConcreteAt(row); ok {
				e.labels[model.LabelName(sanitizeName(c))] = model.LabelValue(valueString(fld, row))
			}
		}
		if _, set := e.labels["level"]; !set && levelField != nil {
			if _, ok := levelField.ConcreteAt(row); ok {
				e.labels["level"] = model.LabelValue(valueString(levelField, row))
			}
		}
		for _, c := range l.structuredMetadataColumns {
			fld := fieldByName(f, c)
			if fld == nil {
				continue
			}
			if _, ok := fld.ConcreteAt(row); ok {
				e.structuredMetadata = append(e.structuredMetadata, push.LabelAdapter{Name: c, Value: valueString(fld, row)})
			}
		}
		e.labels[model.JobLabel] = model.LabelValue(job)
		e.labels[model.InstanceLabel] = model.LabelValue(instance)
		entries = append(entries, e)
	}
	return entries, nil
}

func firstTimeField(f *data.Frame) *data.Field {
	for _, fld := range f.Fields {
		if fld.Type().NonNullableType() == data.FieldTypeTime {
			return fld
		}
	}
	return nil
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
			v = cv
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
