package loghttp

// KEEP IN SYNC WITH:
// https://github.com/grafana/loki/blob/main/pkg/loghttp/labels.go
// Local modifications should be minimized.

import (
	"sort"
	"strconv"
	"strings"

	"github.com/grafana/jsonparser"
)

// LabelSet is a key/value pair mapping of labels
type LabelSet map[string]string

func (l *LabelSet) UnmarshalJSON(data []byte) error {
	if *l == nil {
		*l = make(LabelSet)
	}
	return jsonparser.ObjectEach(data, func(key, val []byte, _ jsonparser.ValueType, _ int) error {
		v, err := jsonparser.ParseString(val)
		if err != nil {
			return err
		}
		k, err := jsonparser.ParseString(key)
		if err != nil {
			return err
		}
		(*l)[k] = v
		return nil
	})
}

// String implements the Stringer interface.  It returns a formatted/sorted set of label key/value pairs.
func (l LabelSet) String() string {
	var b strings.Builder

	keys := make([]string, 0, len(l))
	for k := range l {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l[k]))
	}
	b.WriteByte('}')
	return b.String()
}
