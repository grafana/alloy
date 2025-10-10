package api

import (
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

// Entry is a log entry with labels.
type Entry struct {
	Labels model.LabelSet
	push.Entry
}

// EntryHandler is something that can "handle" entries via a channel.
// Stop must be called to gracefully shutdown the EntryHandler
type EntryHandler interface {
	Chan() chan<- Entry
	Stop()
}
