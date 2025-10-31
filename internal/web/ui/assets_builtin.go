//go:build builtinassets

package ui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:generate npm install
//go:generate npm run build

//go:embed dist
var builtinAssetsTarball embed.FS

// Assets contains the UI's assets.
func Assets() http.FileSystem {
	inner, err := fs.Sub(builtinAssetsTarball, "dist")
	if err != nil {
		panic(err)
	}
	return http.FS(inner)
}
