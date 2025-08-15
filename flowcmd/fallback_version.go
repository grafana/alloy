package flowcmd

import (
	"bufio"
	"bytes"
	_ "embed"
	"strings"

	"github.com/grafana/alloy/internal/build"
)

//go:embed VERSION
var fallbackVersionText []byte

// fallbackVersion returns a version string to use for when the version isn't
// explicitly set at build time. The version string will always have -devel
// appended to it.
func fallbackVersion() string {
	return fallbackVersionFromText(fallbackVersionText)
}

func fallbackVersionFromText(text []byte) string {
	// Find the first line in fallbackVersionText which isn't a blank line or a
	// line starting with #.
	scanner := bufio.NewScanner(bytes.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		return line + "-devel"
	}

	// We shouldn't hit this case since we always control the contents of the
	// VERSION file, but just in case we'll return the existing version.
	return build.Version
}
