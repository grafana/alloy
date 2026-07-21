//go:build !cgo || windows

package fingerprint

// Supported reports whether SQL fingerprinting is available in this build.
// libpg_query requires cgo, so the CGO_ENABLED=0 cross-compile target gets
// stubs that keep the rest of the codebase building without it. Callers that
// need real fingerprints must check Supported() and fail loudly.
func Supported() bool { return false }

func Fingerprint(query string) (string, error) {
	return "", ErrEmpty
}

func FingerprintOf(text string) string { return "" }
