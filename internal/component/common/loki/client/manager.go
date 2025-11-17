package client

import "github.com/grafana/alloy/internal/component/common/loki"

type Manager interface {
	Chan() chan<- loki.Entry
	Stop()
	StopAndDrain()
}
