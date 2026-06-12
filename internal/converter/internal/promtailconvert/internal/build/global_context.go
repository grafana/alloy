package build

import (
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/promtail/limit"
)

type GlobalContext struct {
	WriteReceivers   []loki.Consumer
	TargetSyncPeriod time.Duration
	LabelPrefix      string
	LimitsConfig     limit.Config
}
