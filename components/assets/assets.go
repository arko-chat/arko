package assets

import (
	"encoding/json"
	"io/fs"
	"sync"
)

type ManifestEntry struct {
	File string   `json:"file"`
	Src  string   `json:"src"`
	CSS  []string `json:"css,omitempty"`
}

type Resolver struct {
	once     sync.Once
	manifest map[string]ManifestEntry
	fs       fs.FS
	prefix   string
	dev      bool
	devURL   string
}

var Global *Resolver

func NewResolver(fsys fs.FS, prefix string) *Resolver {
	return &Resolver{fs: fsys, prefix: prefix}
}

func (r *Resolver) SetDev(devURL string) {
	r.dev = true
	r.devURL = devURL
}

func (r *Resolver) IsDev() bool {
	return r.dev
}

func (r *Resolver) ViteClientURL() string {
	if r.dev {
		return "/@vite/client"
	}
	return ""
}

func (r *Resolver) load() {
	r.once.Do(func() {
		f, err := r.fs.Open(".vite/manifest.json")
		if err != nil {
			r.manifest = make(map[string]ManifestEntry)
			return
		}
		defer f.Close()
		json.NewDecoder(f).Decode(&r.manifest)
	})
}

func (r *Resolver) JS(name string) string {
	if r.dev {
		return "/" + name
	}
	r.load()
	if entry, ok := r.manifest[name]; ok {
		return r.prefix + entry.File
	}
	return ""
}

func (r *Resolver) CSS(name string) []string {
	if r.dev {
		return []string{"/" + name}
	}
	r.load()
	if entry, ok := r.manifest[name]; ok {
		if len(entry.CSS) > 0 {
			paths := make([]string, len(entry.CSS))
			for i, c := range entry.CSS {
				paths[i] = r.prefix + c
			}
			return paths
		}
		if entry.File != "" {
			return []string{r.prefix + entry.File}
		}
	}
	return nil
}

func JS(name string) string    { return Global.JS(name) }
func CSS(name string) []string { return Global.CSS(name) }
func IsDev() bool              { return Global.IsDev() }
func ViteClientURL() string    { return Global.ViteClientURL() }
