package opamp

import (
	"context"

	"github.com/open-telemetry/opamp-go/client/types"
	"go.uber.org/zap"
)

var _ types.Logger = (*opAMPLogger)(nil)

type opAMPLogger struct {
	l *zap.SugaredLogger
}

func (o *opAMPLogger) Debugf(_ context.Context, format string, v ...any) {
	o.l.Debugf(format, v...)
}

func (o *opAMPLogger) Errorf(_ context.Context, format string, v ...any) {
	o.l.Errorf(format, v...)
}

func newOpAMPLogger(l *zap.Logger) types.Logger {
	if l == nil {
		l = zap.NewNop()
	}
	return &opAMPLogger{l: l.Sugar()}
}
