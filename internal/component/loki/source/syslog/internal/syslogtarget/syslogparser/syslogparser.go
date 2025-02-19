package syslogparser

import (
	"bufio"
	"fmt"
	"io"

	"github.com/leodido/go-syslog/v4"
	"github.com/leodido/go-syslog/v4/nontransparent"
	"github.com/leodido/go-syslog/v4/octetcounting"
	"github.com/leodido/go-syslog/v4/rfc3164"
	"github.com/leodido/go-syslog/v4/rfc5424"
)

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

	if b == '<' {
		if isRFC3164Message {
			nontransparent.NewParserRFC3164(syslog.WithListener(callback), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithMachineOptions(getRFC3164MachineOptions(useRFC3164DefaultYear)...)).Parse(buf)
		} else {
			nontransparent.NewParser(syslog.WithListener(callback), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithMachineOptions(rfc5424.WithBestEffort())).Parse(buf)
		}
	} else if b >= '0' && b <= '9' {
		if isRFC3164Message {
			octetcounting.NewParserRFC3164(syslog.WithListener(callback), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithMachineOptions(getRFC3164MachineOptions(useRFC3164DefaultYear)...)).Parse(buf)
		} else {
			octetcounting.NewParser(syslog.WithListener(callback), syslog.WithMaxMessageLength(maxMessageLength), syslog.WithMachineOptions(rfc5424.WithBestEffort())).Parse(buf)
		}
	} else {
		return fmt.Errorf("invalid or unsupported framing. first byte: '%s'", string(b))
	}

	return nil
}

func getRFC3164MachineOptions(useRFC3164DefaultYear bool) []syslog.MachineOption {
	options := []syslog.MachineOption{rfc3164.WithBestEffort()}
	if useRFC3164DefaultYear {
		options = append(options, rfc3164.WithYear(rfc3164.CurrentYear{}))
	}
	return options
}
