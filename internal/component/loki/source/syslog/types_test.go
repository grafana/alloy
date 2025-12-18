package syslog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/loki/source/syslog/config"
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
			errSubstring: "unknown syslog format",
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

func TestValidateRawOnlyOpts(t *testing.T) {
	t.Run("RFCFieldsWithNoEffect", func(t *testing.T) {
		sc := &ListenerConfig{
			ListenProtocol: "udp",
			SyslogFormat:   config.SyslogFormatRaw,
		}

		mappings := map[string]*bool{
			"use_rfc5424_message":             &sc.UseRFC5424Message,
			"rfc3164_default_to_current_year": &sc.RFC3164DefaultToCurrentYear,
			"use_incoming_timestamp":          &sc.UseIncomingTimestamp,
		}

		for prop, ptr := range mappings {
			*ptr = true
			err := sc.Validate()
			require.ErrorContains(t, err, prop)
			*ptr = false
		}
	})

	t.Run("RawFormatOptsRequresSyslogFormat", func(t *testing.T) {
		sc := &ListenerConfig{
			ListenProtocol: "udp",
			SyslogFormat:   config.SyslogFormatRFC5424,
			RawFormatOptions: &RawFormatOptions{
				UseNullTerminatorDelimiter: true,
			},
		}

		err := sc.Validate()
		require.ErrorContains(t, err, "raw_format_options has no effect")
	})
}
