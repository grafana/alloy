// Package logutil implements an adaptor for the go-kit logger, which is used in the
// Grafana Agent project, and go-logr, which is used in controller-runtime.
package logutil

import (
	"context"
	"fmt"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

// Wrap wraps a log.Logger into a logr.Logger.
func Wrap(l log.Logger) logr.Logger {
	return logr.New(&goKitLogger{l: l})
}

// FromContext returns a log.Logger from a context. Panics if the context doesn't
// have a Logger set.
func FromContext(ctx context.Context, kvps ...interface{}) log.Logger {
	gkl := clog.FromContext(ctx, kvps...).GetSink().(*goKitLogger)
	return gkl.namedLogger()
}

type goKitLogger struct {
	// name is a name field used by logr which can be appended to dynamically.
	name string
	kvps []interface{}
	l    log.Logger
}

var _ logr.LogSink = (*goKitLogger)(nil)

func (l *goKitLogger) Init(info logr.RuntimeInfo) {
	// no-op
}

func (l *goKitLogger) Enabled(level int) bool { return true }

func (l *goKitLogger) Info(logLevel int, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"msg", msg}, keysAndValues...)
	level.Info(l.namedLogger()).Log(args...)
}

func (l *goKitLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	args := append([]interface{}{"msg", msg, "err", err}, keysAndValues...)
	level.Error(l.namedLogger()).Log(args...)
}

func (l *goKitLogger) WithValues(keysAndValues ...interface{}) logr.LogSink {
	// fix for logs showing "unsupported value type for object references"
	if len(keysAndValues) == 2 {
		if v, ok := keysAndValues[1].(klog.ObjectRef); ok {
			keysAndValues[1] = fmt.Sprintf("%s/%s", v.Namespace, v.Name)
		}
	}
	return &goKitLogger{name: l.name, l: l.l, kvps: append(l.kvps, keysAndValues...)}
}

// namedLogger gets log.Logger with component applied.
func (l *goKitLogger) namedLogger() log.Logger {
	logger := l.l
	if l.name != "" {
		logger = log.With(logger, "component", l.name)
	}
	logger = log.With(logger, l.kvps...)
	return logger
}

func (l *goKitLogger) WithName(name string) logr.LogSink {
	newName := name
	if l.name != "" {
		newName = l.name + "." + name
	}
	return &goKitLogger{name: newName, l: l.l}
}
