package handlers

import (
	"io"
	"net/http"
	"strings"
)

func (h *Handler) HandleProxyMedia(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	mediaURL := strings.TrimRight(sess.Homeserver, "/") + mediaPath

	req, err := http.NewRequestWithContext(r.Context(), "GET", mediaURL, nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Header.Set("Authorization", "Bearer "+sess.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
