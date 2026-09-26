package source

import (
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestFrameToEntriesLineColumn(t *testing.T) {
	ts := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	f := data.NewFrame("q",
		data.NewField("ts", nil, []time.Time{ts}),
		data.NewField("body", nil, []string{"disk full"}),
		data.NewField("type", nil, []string{"alert"}),
		data.NewField("actor", nil, []string{"bob"}),
		data.NewField("severity", nil, []string{"error"}),
	)
	l := logsSpec{lineColumn: "body", labelColumns: []string{"type"}, structuredMetadataColumns: []string{"actor"}}

	got, err := frameToEntries(f, "j", "i", l, time.Now())
	require.NoError(t, err)
	require.Equal(t, []entry{{
		ts:                 ts,
		line:               "disk full",
		labels:             model.LabelSet{"job": "j", "instance": "i", "type": "alert", "level": "error"},
		structuredMetadata: push.LabelsAdapter{{Name: "actor", Value: "bob"}},
	}}, got)
}

func TestFrameToEntriesRowJSONAndPollTime(t *testing.T) {
	now := time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)
	f := data.NewFrame("q",
		data.NewField("msg", nil, []string{"hi"}),
		data.NewField("n", nil, []*float64{nil}),
	)

	got, err := frameToEntries(f, "j", "i", logsSpec{lineColumn: "body"}, now)
	require.NoError(t, err)
	require.Len(t, got, 1)
	require.Equal(t, now, got[0].ts)
	require.JSONEq(t, `{"msg":"hi","n":null}`, got[0].line)
	require.Equal(t, `{"msg":"hi","n":null}`, got[0].line, "keys keep column order")
}

func TestFrameToEntriesTimestampFallsBackToSecondTimeColumn(t *testing.T) {
	primary := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	fallback := time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)
	f := data.NewFrame("q",
		data.NewField("body", nil, []string{"a", "b"}),
		data.NewField("ts1", nil, []*time.Time{nil, &primary}),
		data.NewField("ts2", nil, []*time.Time{&fallback, &fallback}),
	)

	got, err := frameToEntries(f, "j", "i", logsSpec{lineColumn: "body"}, time.Now())
	require.NoError(t, err)
	require.Len(t, got, 2)
	// Row 0: ts1 is nil, so the entry falls back to ts2.
	require.Equal(t, fallback, got[0].ts)
	// Row 1: ts1 has a value, so it wins over ts2.
	require.Equal(t, primary, got[1].ts)
}

func TestFrameToEntriesLevelLabelWins(t *testing.T) {
	f := data.NewFrame("q",
		data.NewField("body", nil, []string{"x"}),
		data.NewField("level", nil, []string{"info"}),
		data.NewField("severity", nil, []string{"error"}),
	)

	got, err := frameToEntries(f, "j", "i", logsSpec{lineColumn: "body", labelColumns: []string{"level"}}, time.Now())
	require.NoError(t, err)
	require.Equal(t, model.LabelValue("info"), got[0].labels["level"])
}

func TestFrameToEntriesEntryLimit(t *testing.T) {
	f := data.NewFrame("q", data.NewField("body", nil, []string{"a", "b", "c"}))

	_, err := frameToEntries(f, "j", "i", logsSpec{lineColumn: "body", entryLimit: 2}, time.Now())
	require.Equal(t, reasonEntryLimit, reasonOf(err))
}
