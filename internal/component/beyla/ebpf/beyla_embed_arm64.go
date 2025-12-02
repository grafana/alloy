//go:build linux && arm64

package beyla

import (
	_ "embed"
)

// Embed the Beyla binary for ARM64
// This file is downloaded from https://github.com/grafana/beyla/releases
//go:embed beyla_binary_arm64
var beylaEmbeddedBinary []byte

