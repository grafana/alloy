package logging_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestUpdateLevel(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	baseLogger, err := logging.New(buffer, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	gkLogger := log.With(baseLogger, "test", "test")
	gkLogger.Log("msg", "hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp := strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=info msg=hello test=test\n", noTimestamp)

	sLogger := slog.New(logging.NewSlogGoKitHandler(gkLogger))
	buffer.Reset()
	sLogger.Info("hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp = strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=info msg=hello test=test\n", noTimestamp)

	buffer.Reset()
	sLogger.Debug("hello")
	require.Equal(t, "", buffer.String())

	err = baseLogger.Update(logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	buffer.Reset()
	sLogger.Info("hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp = strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=info msg=hello test=test\n", noTimestamp)

	buffer.Reset()
	sLogger.Debug("hello")
	require.Contains(t, buffer.String(), "ts=")
	noTimestamp = strings.Join(strings.Split(buffer.String(), " ")[1:], " ")
	require.Equal(t, "level=debug msg=hello test=test\n", noTimestamp)
}
