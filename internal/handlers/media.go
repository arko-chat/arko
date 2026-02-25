package handlers

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/arko-chat/arko/internal/cache"
)

type MediaResponse struct {
	ContentType string
	Body        []byte
	StatusCode  int
}

const MaxCacheableMediaSize = 3 * 1024 * 1024

var errMediaTooLarge = errors.New("media too large for cache")

func (h *Handler) HandleProxyMedia(w http.ResponseWriter, r *http.Request) {
	sess := h.session(r)
	if sess == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	mediaPath := r.URL.Query().Get("path")
	if mediaPath == "" {
		http.Error(w, "missing path param", http.StatusBadRequest)
		return
	}

	if !strings.HasPrefix(mediaPath, "/_matrix/client/v1/media/") {
		http.Error(w, "forbidden path", http.StatusForbidden)
		return
	}

	media, err := cache.CachedSingleWithTTL(
		h.mediaCache,
		h.mediaSfg,
		"hpm:"+mediaPath,
		24*time.Hour,
		func() (*MediaResponse, error) {
			mediaURL := strings.TrimRight(sess.Homeserver, "/") + mediaPath
			req, err := http.NewRequestWithContext(r.Context(), "GET", mediaURL, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+sess.AccessToken)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return nil, err
			}
			defer resp.Body.Close()

			if resp.ContentLength > MaxCacheableMediaSize {
				return nil, errMediaTooLarge
			}

			lr := io.LimitReader(resp.Body, MaxCacheableMediaSize+1)
			body, err := io.ReadAll(lr)
			if err != nil {
				return nil, err
			}

			if len(body) > MaxCacheableMediaSize {
				return nil, errMediaTooLarge
			}

			return &MediaResponse{
				ContentType: resp.Header.Get("Content-Type"),
				Body:        body,
				StatusCode:  resp.StatusCode,
			}, nil
		},
	)

	if err != nil {
		if errors.Is(err, errMediaTooLarge) {
			h.proxyLargeMedia(w, r, sess.Homeserver, sess.AccessToken, mediaPath)
			return
		}
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", media.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(media.StatusCode)
	w.Write(media.Body)
}

func (h *Handler) proxyLargeMedia(w http.ResponseWriter, r *http.Request, hs, token, path string) {
	mediaURL := strings.TrimRight(hs, "/") + path
	req, _ := http.NewRequestWithContext(r.Context(), "GET", mediaURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
