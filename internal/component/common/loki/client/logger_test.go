package client

import (
	"net/url"
	"testing"
	"time"

	cortexflag "github.com/grafana/dskit/flagext"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/agent/internal/component/common/loki"
	"github.com/grafana/loki/pkg/logproto"
	util_log "github.com/grafana/loki/pkg/util/log"
)

func TestNewLogger(t *testing.T) {
	_, err := NewLogger(nilMetrics, util_log.Logger, []Config{}...)
	require.Error(t, err)

	l, err := NewLogger(nilMetrics, util_log.Logger, []Config{{URL: cortexflag.URLValue{URL: &url.URL{Host: "string"}}}}...)
	require.NoError(t, err)
	l.Chan() <- loki.Entry{Labels: model.LabelSet{"foo": "bar"}, Entry: logproto.Entry{Timestamp: time.Now(), Line: "entry"}}
	l.Stop()
}
