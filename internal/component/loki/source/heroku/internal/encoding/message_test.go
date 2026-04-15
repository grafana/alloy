package encoding

import (
	"testing"
	"time"

	"errors"
)

func TestMessageSize(t *testing.T) {
	lockedDate, _ := time.Parse("2006-01-02T15:04:05.000Z", "2019-01-12T11:45:26.371Z")

	tests := map[string]struct {
		msg     Message
		msgSize int
		err     error
	}{
		"happy": {
			msg: Message{
				Version:     1,
				Priority:    134,
				Hostname:    "hostname",
				Application: "application",
				Process:     "process",
				ID:          "msgid",
				Timestamp:   lockedDate,
				Message:     "hi",
			},
			msgSize: 77, // "74 <encoded msg>"
		},

		"invalid-msg": {
			msg: Message{
				Version: 0,
			},
			err: ErrInvalidMessage,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			size, err := test.msg.Size()
			if !errors.Is(err, test.err) {
				t.Errorf("expected %v, got %v", test.err, err)
			}

			if size != test.msgSize {
				t.Errorf("expected %v, got %v", test.msgSize, size)
			}
		})
	}
}
