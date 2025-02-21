package logging_test

import (
	"bytes"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/go-kit/log"
	"github.com/go-logfmt/logfmt"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestWithSlogTester(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	handler := logging.NewSlogGoKitHandler(log.NewLogfmtLogger(buffer))

	err := slogtest.TestHandler(handler, func() []map[string]any {
		results := []map[string]any{}

		dec := logfmt.NewDecoder(buffer)
		for dec.ScanRecord() {
			res := map[string]any{}
			for dec.ScanKeyval() {
				k, v, err := parseValue(string(dec.Key()), dec.Value())
				require.NoError(t, err)
				// If it's a map, merge it with the current map
				if res[k] != nil && reflect.TypeOf(res[k]).Kind() == reflect.Map {
					res[k] = mergeMaps(res[k].(map[string]any), v.(map[string]any))
					continue
				}
				res[k] = v
			}
			results = append(results, res)
		}

		require.NoError(t, dec.Err())
		return results
	})
	require.NoError(t, err)
}

func mergeMaps(m1, m2 map[string]any) map[string]any {
	for k, v := range m2 {
		if m1[k] != nil && reflect.TypeOf(m1[k]).Kind() == reflect.Map {
			m1[k] = mergeMaps(m1[k].(map[string]any), v.(map[string]any))
			continue
		}
		m1[k] = v
	}
	return m1
}

func parseValue(key string, value []byte) (string, any, error) {
	switch key {
	case "level":
		var l slog.Level
		err := l.UnmarshalText([]byte(value))
		if err != nil {
			return key, nil, err
		}
		return key, l, nil
	case "time":
		// parse timestamp in iso8601 2025-02-20T16:58:30.683457-05:00
		parsedTime, err := time.Parse(time.RFC3339Nano, string(value))
		if err != nil {
			return key, nil, err
		}
		return key, parsedTime, nil
	}

	groups := strings.SplitN(key, ".", 2)
	if len(groups) != 2 {
		return key, string(value), nil
	}

	k, v, err := parseValue(groups[1], value)
	if err != nil {
		return key, nil, err
	}

	return groups[0], map[string]any{k: v}, nil
}

func TestUpdateLevel(t *testing.T) {
	buffer := bytes.NewBuffer(nil)
	baseLogger, err := logging.New(buffer, logging.Options{Level: logging.LevelInfo, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	gkLogger := log.With(baseLogger, "test", "test")
	gkLogger.Log("msg", "hello")
	require.Contains(t, buffer.String(), "ts=")
	require.Equal(t, "level=info msg=hello test=test\n", removeTimestamp(buffer.String()))

	sLogger := slog.New(logging.NewSlogGoKitHandler(gkLogger))
	buffer.Reset()
	sLogger.Info("hello")
	require.Contains(t, buffer.String(), "ts=")
	require.Equal(t, "level=info msg=hello test=test\n", removeTimestamp(buffer.String()))

	buffer.Reset()
	sLogger.Debug("hello")
	require.Equal(t, "", buffer.String())

	err = baseLogger.Update(logging.Options{Level: logging.LevelDebug, Format: logging.FormatLogfmt})
	require.NoError(t, err)

	buffer.Reset()
	sLogger.Info("hello")
	require.Contains(t, buffer.String(), "ts=")
	require.Equal(t, "level=info msg=hello test=test\n", removeTimestamp(buffer.String()))

	buffer.Reset()
	sLogger.Debug("hello")
	require.Contains(t, buffer.String(), "ts=")
	require.Equal(t, "level=debug msg=hello test=test\n", removeTimestamp(buffer.String()))
}

func removeTimestamp(s string) string {
	parts := strings.Split(s, " ")
	newParts := make([]string, 0, len(parts)-1)
	for _, p := range parts {
		if strings.Contains(p, "ts=") {
			continue
		}
		newParts = append(newParts, p)
	}
	rejoined := strings.Join(newParts, " ")
	if !strings.HasSuffix(rejoined, "\n") {
		rejoined += "\n"
	}
	return rejoined
}
