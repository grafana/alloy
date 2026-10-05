package graph

import (
	"strings"

	"github.com/grafana/alloy/internal/service/graphql/graph/model"
)

const maxLogsPageSize = 1000

func filterLogLines(lines []string, levels []model.LogLevel) []string {
	wanted := make(map[string]struct{}, len(levels))
	for _, level := range levels {
		wanted[strings.ToLower(string(level))] = struct{}{}
	}

	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if hasLogLevel(line, wanted) {
			filtered = append(filtered, line)
		}
	}
	return filtered
}

func hasLogLevel(line string, wanted map[string]struct{}) bool {
	for _, field := range strings.Fields(line) {
		if level, ok := strings.CutPrefix(field, "level="); ok {
			_, ok := wanted[level]
			return ok
		}
	}

	for level := range wanted {
		if strings.Contains(line, `"level":"`+level+`"`) {
			return true
		}
	}
	return false
}
