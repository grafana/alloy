package kafkatarget

import (
	"testing"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/stretchr/testify/require"
)

func TestFormat_DropActionReturnsEmpty(t *testing.T) {
	lbs := labels.FromStrings("env", "prod", "team", "ignore-me")
	cfg := []*relabel.Config{{
		SourceLabels: []model.LabelName{"team"},
		Regex:        relabel.MustNewRegexp("ignore-.*"),
		Action:       relabel.Drop,
	}}

	require.Empty(t, format(lbs, cfg))
}

func TestFormat_KeepActionReturnsLabels(t *testing.T) {
	lbs := labels.FromStrings("env", "prod", "team", "loki")
	cfg := []*relabel.Config{{
		SourceLabels: []model.LabelName{"team"},
		Regex:        relabel.MustNewRegexp("ignore-.*"),
		Action:       relabel.Drop,
	}}

	require.Equal(t, model.LabelSet{
		"env":  "prod",
		"team": "loki",
	}, format(lbs, cfg))
}
