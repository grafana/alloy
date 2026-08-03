//go:build linux && amd64

package beyla

import "embed"

//go:embed binaries/amd64
var beylaBinaryFS embed.FS
