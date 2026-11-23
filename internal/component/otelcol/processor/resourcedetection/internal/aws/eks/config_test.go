package eks

import "testing"

func TestConvertSetsServiceAccountAuthType(t *testing.T) {
	got := DefaultArguments.Convert()
	if got["auth_type"] != "serviceAccount" {
		t.Fatalf("auth_type = %#v, want serviceAccount", got["auth_type"])
	}
}
