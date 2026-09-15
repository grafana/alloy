package gather

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/featuregate"
)

// FeatureGates writes the state of every feature gate to the bundle.
type FeatureGates struct{}

func (FeatureGates) Name() string { return "feature-gates" }

func (FeatureGates) Gather(_ context.Context, _ Options) ([]File, error) {
	var lines []string
	featuregate.GlobalRegistry().VisitAll(func(g *featuregate.Gate) {
		lines = append(lines, fmt.Sprintf("%s=%t", g.ID(), g.IsEnabled()))
	})

	if len(lines) == 0 {
		return nil, nil
	}

	sort.Strings(lines)
	content := strings.Join(lines, "\n") + "\n"
	return []File{{Path: "feature-gates.txt", Content: []byte(content)}}, nil
}
