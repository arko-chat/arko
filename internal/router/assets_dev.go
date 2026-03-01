//go:build dev

package router

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/arko-chat/arko/components/assets"
	"github.com/go-chi/chi/v5"
)

func registerDevRoutes(r *chi.Mux) {
	target, _ := url.Parse(assets.Global.DevURL())
	proxy := httputil.NewSingleHostReverseProxy(target)

	wsProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "ws"
			req.URL.Host = target.Host
		},
	}

	r.Handle("/@vite/*", proxy)
	r.Handle("/src/*", proxy)
	r.Handle("/node_modules/*", proxy)
	r.Handle("/__vite_ping", proxy)
	_ = wsProxy
}
