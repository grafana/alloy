package syslogparser_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/loki/source/syslog/internal/syslogtarget/syslogparser"
	"github.com/grafana/alloy/internal/util"
	"github.com/leodido/go-syslog/v4"
	"github.com/leodido/go-syslog/v4/rfc3164"
	"github.com/leodido/go-syslog/v4/rfc5424"
	"github.com/stretchr/testify/require"
)

var (
	defaultMaxMessageLength = 8192
)

func TestParseStream_OctetCounting(t *testing.T) {
	r := strings.NewReader("23 <13>1 - - - - - - First24 <13>1 - - - - - - Second")

	results := make([]*syslog.Result, 0)
	cb := func(res *syslog.Result) {
		results = append(results, res)
	}

	err := syslogparser.ParseStream(false, false, r, cb, defaultMaxMessageLength)
	require.NoError(t, err)

	require.Equal(t, 2, len(results))
	require.NoError(t, results[0].Error)
	require.Equal(t, "First", *results[0].Message.(*rfc5424.SyslogMessage).Message)
	require.NoError(t, results[1].Error)
	require.Equal(t, "Second", *results[1].Message.(*rfc5424.SyslogMessage).Message)
}

func TestParseStream_ValidParseError(t *testing.T) {
	// This message can not parse fully but is valid when using the BestEffort Parser Option.
	r := strings.NewReader("17 <13>1       First")

	results := make([]*syslog.Result, 0)
	cb := func(res *syslog.Result) {
		results = append(results, res)
	}

	err := syslogparser.ParseStream(false, false, r, cb, defaultMaxMessageLength)
	require.NoError(t, err)

	require.Equal(t, 1, len(results))
	require.EqualError(t, results[0].Error, "expecting a RFC3339MICRO timestamp or a nil value [col 6]")
	require.True(t, results[0].Message.(*rfc5424.SyslogMessage).Valid())
}

func TestParseStream_OctetCounting_LongMessage(t *testing.T) {
	r := strings.NewReader("8198 <13>1 - - - - - - First")

	results := make([]*syslog.Result, 0)
	cb := func(res *syslog.Result) {
		results = append(results, res)
	}

	err := syslogparser.ParseStream(false, false, r, cb, defaultMaxMessageLength)
	require.NoError(t, err)

	require.Equal(t, 1, len(results))
	require.EqualError(t, results[0].Error, "message length (8198) exceeds maximum length (8192)")
}

func TestParseStream_NewlineSeparated(t *testing.T) {
	r := strings.NewReader("<13>1 - - - - - - First\n<13>1 - - - - - - Second\n")

	results := make([]*syslog.Result, 0)
	cb := func(res *syslog.Result) {
		results = append(results, res)
	}

	err := syslogparser.ParseStream(false, false, r, cb, defaultMaxMessageLength)
	require.NoError(t, err)

	require.Equal(t, 2, len(results))
	require.NoError(t, results[0].Error)
	require.Equal(t, "First", *results[0].Message.(*rfc5424.SyslogMessage).Message)
	require.NoError(t, results[1].Error)
	require.Equal(t, "Second", *results[1].Message.(*rfc5424.SyslogMessage).Message)
}

func TestParseStream_InvalidStream(t *testing.T) {
	r := strings.NewReader("invalid")

	err := syslogparser.ParseStream(false, false, r, func(_ *syslog.Result) {}, defaultMaxMessageLength)
	require.EqualError(t, err, "invalid or unsupported framing. first byte: 'i'")
}

func TestParseStream_EmptyStream(t *testing.T) {
	r := strings.NewReader("")

	err := syslogparser.ParseStream(false, false, r, func(_ *syslog.Result) {}, defaultMaxMessageLength)
	require.Equal(t, err, io.EOF)
}

func TestParseStream_RFC3164Timestamp(t *testing.T) {
	r := strings.NewReader("<13>Dec  1 00:00:00 host Message")

	results := make([]*syslog.Result, 0)
	cb := func(res *syslog.Result) {
		results = append(results, res)
	}

	err := syslogparser.ParseStream(true, false, r, cb, defaultMaxMessageLength)
	require.NoError(t, err)

	require.Equal(t, 1, len(results))
	require.NoError(t, results[0].Error)
	require.Equal(t, "Message", *results[0].Message.(*rfc3164.SyslogMessage).Message)
	require.Equal(t, "host", *results[0].Message.(*rfc3164.SyslogMessage).Hostname)
	require.Equal(t, time.Date(0, 12, 1, 0, 0, 0, 0, time.UTC), *results[0].Message.(*rfc3164.SyslogMessage).Timestamp)
}

func TestParseStream_RFC3164TimestampWithYear(t *testing.T) {
	r := strings.NewReader("<13>Dec  1 00:00:00 host Message")

	results := make([]*syslog.Result, 0)
	cb := func(res *syslog.Result) {
		results = append(results, res)
	}

	err := syslogparser.ParseStream(true, true, r, cb, defaultMaxMessageLength)
	require.NoError(t, err)

	require.Equal(t, 1, len(results))
	require.NoError(t, results[0].Error)
	require.Equal(t, "Message", *results[0].Message.(*rfc3164.SyslogMessage).Message)
	require.Equal(t, "host", *results[0].Message.(*rfc3164.SyslogMessage).Hostname)

	// Manually calculate the expected year based on the edge case logic (SetYearForLimitedTimeFormat)
	now := time.Now()
	expectedYear := now.Year()
	if now.Month() == time.January {
		expectedYear = now.Year() - 1
	} else if now.Month() == time.December {
		expectedYear = now.Year() + 1
	}

	require.Equal(t, time.Date(expectedYear, time.December, 1, 0, 0, 0, 0, time.UTC), *results[0].Message.(*rfc3164.SyslogMessage).Timestamp)
}

// Tests the edge case where a December log is parsed in January, which should use the previous year.
func TestParseStream_SetYearForLimitedTimeFormat_DecemberInJanuary(t *testing.T) {
	parsedTime := time.Date(0, time.December, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.January, 15, 12, 0, 0, 0, time.UTC)

	util.SetYearForLimitedTimeFormat(&parsedTime, now)

	expected := time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC)
	require.Equal(t, expected, parsedTime)
}

// tests the edge case where a January log is parsed in December, which should use the next year.
func TestParseStream_SetYearForLimitedTimeFormat_JanuaryInDecember(t *testing.T) {
	parsedTime := time.Date(0, time.January, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2025, time.December, 15, 12, 0, 0, 0, time.UTC)

	util.SetYearForLimitedTimeFormat(&parsedTime, now)

	// January log seen in December should be from the next year
	expected := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	require.Equal(t, expected, parsedTime)
}

// Tests the "normal" case where the parsed month doesn't trigger the New Year's edge case logic.
func TestParseStream_SetYearForLimitedTimeFormat_NormalCase(t *testing.T) {
	parsedTime := time.Date(0, time.June, 15, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.June, 20, 12, 0, 0, 0, time.UTC)

	util.SetYearForLimitedTimeFormat(&parsedTime, now)

	expected := time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)
	require.Equal(t, expected, parsedTime)
}
