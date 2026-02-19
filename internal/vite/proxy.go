package vite

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func NewProxy(viteURL string) http.Handler {
	target, _ := url.Parse(viteURL)
	proxy := httputil.NewSingleHostReverseProxy(target)
	return proxy
}
