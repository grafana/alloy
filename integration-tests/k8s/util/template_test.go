package util

import (
	"strings"
	"testing"
)

func TestSubstituteVars(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		vars    map[string]string
		want    string
		wantErr string
	}{
		{
			name: "no placeholders is identity",
			in:   "apiVersion: v1\nkind: Namespace\n",
			want: "apiVersion: v1\nkind: Namespace\n",
		},
		{
			name: "single substitution",
			in:   "namespace: ${NAMESPACE}\n",
			vars: map[string]string{"NAMESPACE": "test-foo"},
			want: "namespace: test-foo\n",
		},
		{
			name: "multiple occurrences and keys",
			in:   "  - ${NS}\n  - ${OTHER_NS}\n  - ${NS}\n",
			vars: map[string]string{"NS": "a", "OTHER_NS": "b"},
			want: "  - a\n  - b\n  - a\n",
		},
		{
			name: "bare dollar signs are left alone",
			in:   "value: $literal and ${KEY}\n",
			vars: map[string]string{"KEY": "ok"},
			want: "value: $literal and ok\n",
		},
		{
			name:    "missing key fails fast with sorted unique list",
			in:      "${B} ${A} ${A} ${B}",
			wantErr: "unresolved ${VAR} placeholders: [A B]",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SubstituteVars(tc.in, tc.vars)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil (out=%q)", tc.wantErr, got)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tc.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
