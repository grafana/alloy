package util

import (
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

// LogError logs any error returned by f; useful when deferring Close etc.
func LogError(logger log.Logger, message string, f func() error) {
	if err := f(); err != nil {
		level.Error(logger).Log("message", message, "error", err)
	}
}
