package gather

import "testing"

func TestLogRingWraparoundAndSnapshot(t *testing.T) {
	r := newLogRing(10)

	r.Write([]byte("abcde"))
	if data, evicted := r.snapshot(); string(data) != "abcde" || evicted {
		t.Fatalf("got %q evicted=%v", data, evicted)
	}

	r.Write([]byte("fghij")) // fills exactly
	if data, evicted := r.snapshot(); string(data) != "abcdefghij" || !evicted {
		t.Fatalf("got %q evicted=%v", data, evicted)
	}

	r.Write([]byte("KL")) // wraps, evicts "ab"
	if data, _ := r.snapshot(); string(data) != "cdefghijKL" {
		t.Fatalf("got %q", data)
	}

	r.Write([]byte("0123456789XYZ")) // larger than ring: keep last 10
	if data, _ := r.snapshot(); string(data) != "3456789XYZ" {
		t.Fatalf("got %q", data)
	}
}
