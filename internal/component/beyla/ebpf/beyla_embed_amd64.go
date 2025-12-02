//go:build linux && amd64

package beyla

import (
	_ "embed"
)

// Embed the Beyla binary for AMD64
// This file is downloaded from https://github.com/grafana/beyla/releases
//
//go:embed beyla_binary_amd64
var beylaEmbeddedBinary []byte

