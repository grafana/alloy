package encoding

import (
	"errors"
	"fmt"
	"strconv"
)

// SyslogTimeFormat defines the exact time format used in logs.
const SyslogTimeFormat = "2006-01-02T15:04:05.999999-07:00"

// ErrInvalidMessage returned when trying to encode an invalid syslog message
var ErrInvalidMessage = errors.New("invalid message")

// Encode serializes a syslog message into their wire format (octet-framed syslog).
// Disabling RFC 5424 compliance is the default and needed due to https://github.com/heroku/logplex/issues/204
func Encode(msg Message) ([]byte, error) {
	sd := ""
	if msg.RFCCompliant {
		sd = "- "
	}

	if msg.Version == 0 {
		return nil, fmt.Errorf("%w: version", ErrInvalidMessage)
	}

	line := "<" + strconv.Itoa(int(msg.Priority)) + ">" + strconv.Itoa(int(msg.Version)) + " " +
		msg.Timestamp.Format(SyslogTimeFormat) + " " +
		stringOrNil(msg.Hostname) + " " +
		stringOrNil(msg.Application) + " " +
		stringOrNil(msg.Process) + " " +
		stringOrNil(msg.ID) + " " +
		sd +
		msg.Message

	return []byte(strconv.Itoa(len(line)) + " " + line), nil
}

func stringOrNil(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
