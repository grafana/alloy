package client

import (
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	cortexflag "github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/loki/promtail/api"
)

func TestNewLogger(t *testing.T) {
	_, err := NewLogger(nilMetrics, log.NewNopLogger(), []Config{}...)
	require.Error(t, err)

	l, err := NewLogger(nilMetrics, log.NewNopLogger(), []Config{{URL: cortexflag.URLValue{URL: &url.URL{Host: "string"}}}}...)
	require.NoError(t, err)
	l.Chan() <- api.Entry{Labels: model.LabelSet{"foo": "bar"}, Entry: push.Entry{Timestamp: time.Now(), Line: "entry"}}
	l.Stop()
}
