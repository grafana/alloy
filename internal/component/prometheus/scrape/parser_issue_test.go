package scrape

import (
	"encoding/base64"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/stretchr/testify/require"
)

func TestProtobufParse2(t *testing.T) {
	// inputBuf := createTestProtoBuf(t)
	// bb := inputBuf.Bytes()

	// Read test data from file
	bb, err := os.ReadFile("testdata/proto-base64.txt")
	if err != nil {
		t.Fatal(err)
	}
	// Decode base64
	bb, err = base64.StdEncoding.DecodeString(string(bb))
	if err != nil {
		t.Fatal(err)
	}

	scenarios := []struct {
		name     string
		parser   textparse.Parser
		expected []parsedEntry
	}{
		{
			name:   "parse classic and native buckets",
			parser: textparse.NewProtobufParser(bb, true, nil),
		},
		{
			name:   "don't parse classic and native buckets",
			parser: textparse.NewProtobufParser(bb, false, nil),
		},
		{
			name:   "parse classic and native buckets with symbol table",
			parser: textparse.NewProtobufParser(bb, true, labels.NewSymbolTable()),
		},
		{
			name:   "don't parse classic and native buckets with symbol table",
			parser: textparse.NewProtobufParser(bb, false, labels.NewSymbolTable()),
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			p := scenario.parser
			testParse2(t, p)
		})
	}
}

func testParse2(t *testing.T, p textparse.Parser) (ret []parsedEntry) {
	t.Helper()

	var (
		lastLabels    labels.Labels
		lastLabelsStr = "{}"
	)

	for {
		require.True(t, lastLabels.IsValid(model.LegacyValidation))
		require.Equal(t, lastLabelsStr, lastLabels.String())

		et, err := p.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)

		var got parsedEntry
		var m []byte

		require.True(t, lastLabels.IsValid(model.LegacyValidation))
		require.Equal(t, lastLabelsStr, lastLabels.String())

		switch et {
		case textparse.EntryInvalid:
			t.Fatal("entry invalid not expected")
		case textparse.EntrySeries, textparse.EntryHistogram:
			var ts *int64
			if et == textparse.EntrySeries {
				m, ts, got.v = p.Series()
			} else {
				m, ts, got.shs, got.fhs = p.Histogram()
			}
			if ts != nil {
				got.t = nil
			}
			got.m = string(m)

			p.Labels(&lastLabels)
			_ = lastLabels.Hash()
			_ = lastLabels.IsEmpty()
			_ = lastLabels.Has(labels.MetricName)
			require.True(t, lastLabels.IsValid(model.LegacyValidation))

			lb := labels.NewBuilder(lastLabels)
			lb.Set("foo", "bar")
			lb.Set("another", "label")
			lb.Set("zoo", "animal")
			lastLabels = lb.Labels()

			lastLabelsStr = lastLabels.String()
			got.lset = lastLabels
			t.Log(lastLabelsStr)

			got.ct = p.CreatedTimestamp()

			for e := (exemplar.Exemplar{}); p.Exemplar(&e); {
				got.es = append(got.es, e)
			}
		case textparse.EntryType:
			m, got.typ = p.Type()
			got.m = string(m)

		case textparse.EntryHelp:
			m, h := p.Help()
			got.m = string(m)
			got.help = string(h)

		case textparse.EntryUnit:
			m, u := p.Unit()
			got.m = string(m)
			got.unit = string(u)

		case textparse.EntryComment:
			got.comment = string(p.Comment())
		}
		ret = append(ret, got)
	}
	return ret
}

type parsedEntry struct {
	// In all but EntryComment, EntryInvalid.
	m string

	// In EntryHistogram.
	shs *histogram.Histogram
	fhs *histogram.FloatHistogram

	// In EntrySeries.
	v float64

	// In EntrySeries and EntryHistogram.
	lset labels.Labels
	t    *int64
	es   []exemplar.Exemplar
	ct   int64

	// In EntryType.
	typ model.MetricType
	// In EntryHelp.
	help string
	// In EntryUnit.
	unit string
	// In EntryComment.
	comment string
}
