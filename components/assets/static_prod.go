//go:build !dev

package assets

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var staticFiles embed.FS

func DistFS() fs.FS {
	sub, _ := fs.Sub(staticFiles, "dist")
	return sub
}

func init() {
	Global = NewResolver(DistFS(), "")
}
