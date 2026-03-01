//go:build dev

package assets

import (
	"io/fs"
	"os"
)

func DistFS() fs.FS { return nil }

func (r *Resolver) DevURL() string { return r.devURL }

func init() {
	devURL := "http://localhost:5173"
	if env := os.Getenv("VITE_DEV_URL"); env != "" {
		devURL = env
	}
	Global = NewResolver(nil, "")
	Global.SetDev(devURL)
}
