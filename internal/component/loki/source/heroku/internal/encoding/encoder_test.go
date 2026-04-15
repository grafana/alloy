package encoding

import (
	"bytes"
	"io"
	"testing"
	"time"

	"errors"
)

func TestEncode(t *testing.T) {
	lockedDate, _ := time.Parse("2006-01-02T15:04:05.000Z", "2019-01-12T11:45:26.371Z")

	tests := map[string]struct {
		msg     Message
		msgSize int64
		err     error
		writer  io.Writer
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
			writer:  io.Discard,
			msgSize: int64(77), // "74 <encoded msg>"
		},

		"missing-version": {
			msg: Message{
				Priority:    134,
				Hostname:    "hostname",
				Application: "application",
				Process:     "process",
				ID:          "msgid",
				Timestamp:   lockedDate,
				Message:     "hi",
			},
			writer: io.Discard,
			err:    ErrInvalidMessage,
		},

		"short-buffer": {
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
			writer: failWrite{},
			err:    io.EOF,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			n, err := test.msg.WriteTo(test.writer)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected %v, got %#v", test.err, err)
			}

			if n != test.msgSize {
				t.Errorf("invalid encoded length\nexpected: %d, got %d", test.msgSize, n)
			}
		})
	}
}

func TestEncoderTypes(t *testing.T) {
	lockedDate, _ := time.Parse("2006-01-02T15:04:05.000Z", "2019-01-12T11:45:26.371Z")

	tests := []struct {
		name           string
		encoderType    string
		msg            Message
		wantEncodedMsg string
		wantErr        error
	}{
		{
			name:        "successful encoding with plain encoder",
			encoderType: "plain",
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
			wantEncodedMsg: "2019-01-12T11:45:26.371000+00:00 application[process]: hi",
		},
		{
			name:        "successful encoding with SSE encoder",
			encoderType: "sse",
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
			wantEncodedMsg: "id: 1547293526\ndata: 2019-01-12T11:45:26.371000+00:00 application[process]: hi\n\n\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &bytes.Buffer{}
			var e Encoder
			if test.encoderType == "sse" {
				e = NewSSE(w)
			} else {
				e = NewPlain(w)
			}

			err := e.Encode(test.msg)
			if err != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("want error: %v, got: %v", test.wantErr, err)
			}

			// Verify message content even if an error is expected, received message should be empty
			if test.wantEncodedMsg != w.String() {
				t.Fatalf("want message: %v, got: %v", test.wantEncodedMsg, w.String())
			}
		})
	}
}

func BenchmarkMessageToString(b *testing.B) {
	lockedDate, err := time.Parse("2006-01-02T15:04:05.000Z", "2019-01-12T11:45:26.371Z")
	if err != nil {
		b.Fatal("unexpected error parsing benchmark input", err)
	}
	msg := Message{
		Version:     1,
		Priority:    134,
		Hostname:    "hostname",
		Application: "application",
		Process:     "process",
		ID:          "msgid",
		Timestamp:   lockedDate,
		Message:     "hi",
	}
	var result string
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		result = messageToString(msg)
	}
	_ = result
}

func BenchmarkEncode(b *testing.B) {
	lockedDate, err := time.Parse("2006-01-02T15:04:05.000Z", "2019-01-12T11:45:26.371Z")
	if err != nil {
		b.Fatal("unexpected error parsing benchmark input", err)
	}
	msg := Message{
		Version:     1,
		Priority:    134,
		Hostname:    "hostname",
		Application: "application",
		Process:     "process",
		ID:          "msgid",
		Timestamp:   lockedDate,
		Message:     "hi",
	}
	var result []byte
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		result, _ = Encode(msg)
	}
	_ = result
}

type failWrite struct{}

func (failWrite) Write([]byte) (int, error) {
	return 0, io.EOF
}
