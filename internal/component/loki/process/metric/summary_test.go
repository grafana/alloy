package metric

import (
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func TestSummaryExpiration(t *testing.T) {
	t.Parallel()
	cfg := &SummaryConfig{
		Description: "summary test",
		MaxIdle:     1 * time.Second,
	}

	sum, err := NewSummaries("test_summary", cfg)
	assert.Nil(t, err)

	// First label
	lbl1 := model.LabelSet{}
	lbl1["test"] = "app"
	sum.With(lbl1).Observe(10)

	collect(sum)
	assert.Contains(t, sum.metrics, lbl1.Fingerprint())

	time.Sleep(1100 * time.Millisecond)

	// Second label
	lbl2 := model.LabelSet{}
	lbl2["test"] = "app2"
	sum.With(lbl2).Observe(5)

	collect(sum)
	assert.NotContains(t, sum.metrics, lbl1.Fingerprint())
	assert.Contains(t, sum.metrics, lbl2.Fingerprint())
}
