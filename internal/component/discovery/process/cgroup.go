//go:build linux

package process

import (
	"bufio"
	"io"
	"regexp"
)

func getIDFromCGroup(cgroup io.Reader, regexp *regexp.Regexp) string {
	if regexp == nil {
		return ""
	}

	scanner := bufio.NewScanner(cgroup)
	for scanner.Scan() {
		line := scanner.Bytes()
		matches := regexp.FindSubmatch(line)
		if len(matches) <= 1 {
			continue
		}
		return string(matches[1])
	}
	return ""
}
