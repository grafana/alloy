//go:build linux

package process

import (
	"bufio"
	"io"
	"strings"
)

// getPathFromCGroup fetches cgroup path(s) from process.
// In the case of cgroups v2 (unified), there will be only
// one path and function returns that path. In the case
// cgroups v1, there will be one path for each controller.
// The function will join all the paths using `|` and
// returns as one string. Users can use relabel component
// to retrieve the path that they are interested.
func getPathFromCGroup(cgroup io.Reader) string {
	var paths []string
	scanner := bufio.NewScanner(cgroup)
	for scanner.Scan() {
		line := scanner.Bytes()
		paths = append(paths, string(line))
	}
	return strings.Join(paths, "|")
}
