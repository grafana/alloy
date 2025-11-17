package client

import "github.com/grafana/alloy/internal/component/common/loki"

type Consumer interface {
	Chan() chan<- loki.Entry
}

type Stoppable interface {
	Stop()
}

type Drainable interface {
	StopAndDrain()
}

type StoppableConsumer interface {
	Consumer
	Stoppable
}

type DrainableConsumer interface {
	Consumer
	Stoppable
	Drainable
}
