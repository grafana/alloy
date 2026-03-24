package slogadapter

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// GoKit returns a [log.Logger] that writes to the provided slog.Handler.
func GoKit(h slog.Handler) log.Logger {
	return slogAdapter{h}
}

type slogAdapter struct {
	h slog.Handler
}

var (
	_ log.Logger = (*slogAdapter)(nil)
)

// Enabled implements logging.EnabledAware interface.
func (sa slogAdapter) Enabled(ctx context.Context, l slog.Level) bool {
	return sa.h.Enabled(ctx, l)
}

func (sa slogAdapter) Log(kvps ...any) error {
	// Find the log level first, starting with the default.
	recordLevel := slog.LevelInfo
	for i := 0; i < len(kvps)-1; i += 2 {
		if kvps[i] == level.Key() {
			value := kvps[i+1]
			levelValue, _ := value.(level.Value)
			switch levelValue {
			case level.DebugValue():
				recordLevel = slog.LevelDebug
			case level.InfoValue():
				recordLevel = slog.LevelInfo
			case level.WarnValue():
				recordLevel = slog.LevelWarn
			case level.ErrorValue():
				recordLevel = slog.LevelError
			}
			break
		}
	}

	// Do not build the record if the level is not enabled.
	if !sa.h.Enabled(context.Background(), recordLevel) {
		return nil
	}

	// Since there's a pattern of wrapping loggers at different depths in go-kit,
	// we can't consistently know who the caller is, so we set no value for pc
	// here.
	rec := slog.NewRecord(time.Now(), recordLevel, "", 0)

	for i := 0; i < len(kvps); i += 2 {
		var key, value any

		if i+1 < len(kvps) {
			key = kvps[i]
			value = kvps[i+1]
		} else {
			// Mismatched pair
			key = "!BADKEY"
			value = kvps[i]
		}

		if key == "msg" || key == "message" {
			rec.Message = fmt.Sprint(value)
			continue
		}

		if key == level.Key() {
			// Already handled
			continue
		}

		rec.AddAttrs(slog.Attr{
			Key:   fmt.Sprint(key),
			Value: slog.AnyValue(value),
		})
	}

	return sa.h.Handle(context.Background(), rec)
}
