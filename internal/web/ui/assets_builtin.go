//go:build builtinassets

package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:generate yarn
//go:generate yarn run build

//go:embed build
var builtinAssetsTarball embed.FS

// Assets contains the UI's assets.
func Assets() http.FileSystem {
	inner, err := fs.Sub(builtinAssetsTarball, "build")
	if err != nil {
		panic(err)
	}
	return http.FS(inner)
}
