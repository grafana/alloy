package diag

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiagReporting(t *testing.T) {
	var (
		criticalDiagnostic = Diagnostic{
			Severity: SeverityLevelCritical,
			Summary:  "this is a critical diag",
		}
		errorDiagnostic = Diagnostic{
			Severity: SeverityLevelError,
			Summary:  "this is an error diag",
		}
		warnDiagnostic = Diagnostic{
			Severity: SeverityLevelWarn,
			Summary:  "this is a warn diag",
		}
		infoDiagnostic = Diagnostic{
			Severity: SeverityLevelInfo,
			Summary:  "this is an info diag",
		}
	)

	tt := []struct {
		name            string
		diags           Diagnostics
		bypassErrors    bool
		expectedMessage string
	}{
		{
			name:            "Empty",
			diags:           Diagnostics{},
			expectedMessage: successFooter,
		},
		{
			name:            "Critical",
			diags:           Diagnostics{criticalDiagnostic, errorDiagnostic, warnDiagnostic, infoDiagnostic},
			expectedMessage: `(Critical) this is a critical diag` + criticalErrorFooter,
		},
		{
			name:            "Error",
			diags:           Diagnostics{errorDiagnostic, warnDiagnostic, infoDiagnostic},
			expectedMessage: `(Error) this is an error diag` + errorFooter,
		},
		{
			name:         "Bypass Error",
			diags:        Diagnostics{errorDiagnostic, warnDiagnostic, infoDiagnostic},
			bypassErrors: true,
			expectedMessage: `(Error) this is an error diag
(Warning) this is a warn diag
(Info) this is an info diag` + successFooter,
		},
		{
			name:  "Warn",
			diags: Diagnostics{warnDiagnostic, infoDiagnostic},
			expectedMessage: `(Warning) this is a warn diag
(Info) this is an info diag` + successFooter,
		},
		{
			name:            "Info",
			diags:           Diagnostics{infoDiagnostic},
			expectedMessage: `(Info) this is an info diag` + successFooter,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := generateTextReport(&buf, tc.diags, tc.bypassErrors)
			require.NoError(t, err)

			require.Equal(t, tc.expectedMessage, buf.String())
		})
	}
}
