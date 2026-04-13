//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	cryptorand "crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

const (
	maxNamespaceLength = 53
	maxReleaseLength   = 53
	suffixHexLen       = 8
)

var nonAlnumHyphen = regexp.MustCompile(`[^a-z0-9-]+`)

type testRuntime struct {
	testID    string
	namespace string
	release   string
}

func newTestRuntime(testName string) (testRuntime, error) {
	suffix, err := randomHex(suffixHexLen / 2)
	if err != nil {
		return testRuntime{}, err
	}

	base := sanitizeName(testName)
	if base == "" {
		base = "test"
	}
	id := joinName(base, suffix, maxNamespaceLength)
	return testRuntime{
		testID:    id,
		namespace: id,
		release:   id,
	}, nil
}

func randomHex(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := cryptorand.Read(b); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

func sanitizeName(raw string) string {
	s := strings.ToLower(raw)
	s = strings.ReplaceAll(s, "_", "-")
	s = nonAlnumHyphen.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	s = strings.TrimPrefix(s, "k8s-v2-")
	s = strings.TrimPrefix(s, "k8s-v2")
	return s
}

func joinName(base, suffix string, maxLen int) string {
	if base == "" {
		base = "test"
	}
	maxBase := maxLen - len(suffix) - 1
	if maxBase < 1 {
		maxBase = 1
	}
	base = truncateName(base, maxBase)
	return strings.Trim(base+"-"+suffix, "-")
}

func truncateName(in string, maxLen int) string {
	if len(in) <= maxLen {
		return in
	}
	return strings.Trim(in[:maxLen], "-")
}
