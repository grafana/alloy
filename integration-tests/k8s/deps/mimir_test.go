package deps

import "testing"

func TestWithTestName(t *testing.T) {
	tests := []struct {
		name   string
		metric string
		want   string
	}{
		{"bare name", "m", `m{alloy_test_name="x"}`},
		{"one matcher", `m{result="success"}`, `m{result="success",alloy_test_name="x"}`},
		{"two matchers", `m{a="1",b="2"}`, `m{a="1",b="2",alloy_test_name="x"}`},
		{"empty braces", `m{}`, `m{alloy_test_name="x"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := withTestName(tc.metric, "x"); got != tc.want {
				t.Fatalf("withTestName(%q) = %q, want %q", tc.metric, got, tc.want)
			}
		})
	}
}
