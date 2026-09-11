package gather

import (
	"context"
	"os"
	"strings"
)

// RuntimeFlags writes the process command line to the bundle. It shows how the
// collector was started, such as the config paths and feature gate flags.
type RuntimeFlags struct{}

func (RuntimeFlags) Name() string { return "runtime-flags" }

func (RuntimeFlags) Gather(_ context.Context, _ Options) ([]File, error) {
	if len(os.Args) == 0 {
		return nil, nil
	}
	content := strings.Join(os.Args, "\n") + "\n"
	return []File{{Path: "runtime-flags.txt", Content: []byte(content)}}, nil
}
