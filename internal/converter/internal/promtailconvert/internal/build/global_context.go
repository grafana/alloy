package build

import (
	"time"

	"github.com/grafana/agent/internal/component/common/loki"
)

type GlobalContext struct {
	WriteReceivers   []loki.LogsReceiver
	TargetSyncPeriod time.Duration
	LabelPrefix      string
}
