package infinity

import (
	"context"

	gk "github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	backend "github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type loggerHandler struct {
	Logger gk.Logger
	Lvl    backend.Level
}

// Ensure loggerHandler implements backend.Logger
var _ backend.Logger = loggerHandler{}

func (l loggerHandler) Debug(msg string, args ...interface{}) {
	level.Debug(l.Logger).Log(append([]interface{}{"msg", msg}, args)...)
}

func (l loggerHandler) Info(msg string, args ...interface{}) {
	level.Info(l.Logger).Log(append([]interface{}{"msg", msg}, args)...)
}

func (l loggerHandler) Warn(msg string, args ...interface{}) {
	level.Warn(l.Logger).Log(append([]interface{}{"msg", msg}, args)...)
}

func (l loggerHandler) Error(msg string, args ...interface{}) {
	level.Error(l.Logger).Log(append([]interface{}{"msg", msg}, args)...)
}

func (l loggerHandler) With(args ...interface{}) backend.Logger {
	return &loggerHandler{
		Logger: gk.With(l.Logger, args...),
	}
}

func (l loggerHandler) Level() backend.Level {
	return l.Lvl
}

func (l loggerHandler) FromContext(ctx context.Context) backend.Logger {
	// This is not important as the context is only intended to return context relevant to running as a Data Source
	return l
}
