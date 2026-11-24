package deps

import "testing"

func TestWithTestName(t *testing.T) {
	tests := []struct {
		name      string
		metric    string
		want      string
		wantError bool
	}{
		{"bare name", "m", `m{alloy_test_name="x"}`, false},
		{"one matcher", `m{result="success"}`, `m{result="success",alloy_test_name="x"}`, false},
		{"two matchers", `m{a="1",b="2"}`, `m{a="1",b="2",alloy_test_name="x"}`, false},
		{"empty braces", `m{}`, `m{alloy_test_name="x"}`, false},
		{"open brace no close", `m{a="1"}[5m]`, "", true},
		{"trailing comma", `m{a="1",}`, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := withTestName(tc.metric, "x")
			if tc.wantError {
				if err == nil {
					t.Fatalf("withTestName(%q) expected error, got nil", tc.metric)
				}
				return
			}
			if err != nil {
				t.Fatalf("withTestName(%q) unexpected error: %v", tc.metric, err)
			}
			if got != tc.want {
				t.Fatalf("withTestName(%q) = %q, want %q", tc.metric, got, tc.want)
			}
		})
	}
}
