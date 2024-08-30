package syslog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		scFn func(*ListenerConfig)

		// Empty if no error expected, substring of error otherwise.
		errSubstring string
	}{
		{
			name:         "ValidDefault",
			scFn:         func(sc *ListenerConfig) {},
			errSubstring: "",
		},
		{
			name: "InvalidProtocol",
			scFn: func(sc *ListenerConfig) {
				sc.ListenProtocol = "invalid"
			},
			errSubstring: "syslog listener protocol should be",
		},
		{
			name: "InvalidSyslogFormat",
			scFn: func(sc *ListenerConfig) {
				sc.SyslogFormat = "invalid"
			},
			errSubstring: "syslog format should be",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			sc := DefaultListenerConfig
			tt.scFn(&sc)

			err := sc.Validate()
			if tt.errSubstring == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tt.errSubstring)
			}
		})
	}
}
