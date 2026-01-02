package syslogparser

import (
	"bufio"
	"fmt"
	"io"
	"time"

	"github.com/leodido/go-syslog/v4"
	"github.com/leodido/go-syslog/v4/nontransparent"
	"github.com/leodido/go-syslog/v4/octetcounting"
	"github.com/leodido/go-syslog/v4/rfc3164"

	"github.com/grafana/alloy/internal/util"
)

type framingType = uint

const (
	framingTypeUnknown framingType = iota
	framingTypeOctetCounting
	framingTypeNonTransparent
)

// framingTypeFromFirstByte detects framing type from a first byte of syslog line.
// Returns [framingTypeUnknown] on failure.
//
// See https://datatracker.ietf.org/doc/html/rfc6587 for details on message framing.
//
// Note: this method doesn't support CEF logs as they don't have syslog priority prefix.
func framingTypeFromFirstByte(b byte) framingType {
	if b == '<' {
		// Message starts with log severity and no length, should be consumed until '\n' or '\0' character.
		return framingTypeNonTransparent
	}

	if isDigit(b) {
		// Message starts with content length.
		return framingTypeOctetCounting
	}

	return framingTypeUnknown
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// ParseStream parses a rfc5424 syslog stream from the given Reader, calling
// the callback function with the parsed messages. The parser automatically
// detects octet counting.
// The function returns on EOF or unrecoverable errors.
func ParseStream(isRFC3164Message bool, useRFC3164DefaultYear bool, r io.Reader, callback func(res *syslog.Result), maxMessageLength int) error {
	buf := bufio.NewReaderSize(r, 1<<10)

	b, err := buf.ReadByte()
	if err != nil {
		return err
	}
	_ = buf.UnreadByte()
	cb := callback
	if isRFC3164Message && useRFC3164DefaultYear {
		cb = func(res *syslog.Result) {
			if res.Message != nil {
				rfc3164Msg := res.Message.(*rfc3164.SyslogMessage)
				if rfc3164Msg.Timestamp != nil {
					util.SetYearForLimitedTimeFormat(rfc3164Msg.Timestamp, time.Now())
				}
			}
			callback(res)
		}
	}

	// See https://datatracker.ietf.org/doc/html/rfc6587 for details on message framing
	// If a syslog message starts with '<' the first piece of the message is the priority, which means it must use
	// an explicit framing character.
	switch framingTypeFromFirstByte(b) {
	case framingTypeNonTransparent:
		if isRFC3164Message {
			nontransparent.NewParserRFC3164(syslog.WithListener(cb), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithBestEffort()).Parse(buf)
		} else {
			nontransparent.NewParser(syslog.WithListener(cb), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithBestEffort()).Parse(buf)
		}
	case framingTypeOctetCounting:
		// If a syslog message starts with a digit, it must use octet counting, and the first piece of the message is the length
		if isRFC3164Message {
			octetcounting.NewParserRFC3164(syslog.WithListener(cb), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithBestEffort()).Parse(buf)
		} else {
			octetcounting.NewParser(syslog.WithListener(cb), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithBestEffort()).Parse(buf)
		}
	default:
		return fmt.Errorf("invalid or unsupported framing. first byte: %q", b)
	}

	return nil
}
