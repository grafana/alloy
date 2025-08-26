package write

import (
	"fmt"
	"os"
	"strconv"

	"github.com/go-kit/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

func NewLumberjack() (log.Logger, error) {
	lj := &lumberjack.Logger{
		Filename:   os.Getenv("PYROSCOPE_WRITE_TRACE_LOGGER"),
		MaxSize:    50,
		MaxBackups: 200,
		MaxAge:     2,
		Compress:   true,
	}

	if maxBackupsStr := os.Getenv("PYROSCOPE_WRITE_TRACE_LOGGER_MAX_BACKUPS"); maxBackupsStr != "" {
		if maxBackups, err := strconv.Atoi(maxBackupsStr); err == nil && maxBackups >= 0 {
			lj.MaxBackups = maxBackups
		}
	}
	if lj.Filename == "" {
		return nil, fmt.Errorf("no filename provided")
	}

	debugLogger := log.NewLogfmtLogger(log.NewSyncWriter(lj))
	debugLogger = log.With(debugLogger, "ts", log.DefaultTimestampUTC)

	return debugLogger, nil
}
