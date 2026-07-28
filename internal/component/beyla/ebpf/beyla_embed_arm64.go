//go:build linux && arm64

package beyla

import "embed"

//go:embed binaries/arm64
var beylaBinaryFS embed.FS
