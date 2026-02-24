package handlers

import (
	"fmt"
	"net/http"

	"github.com/arko-chat/arko/internal/session"
)

func (h *Handler) HandleToggleTheme(w http.ResponseWriter, r *http.Request) {
	userID := session.ReadCookie(r)
	if userID == "" {
		h.serverError(w, r, fmt.Errorf("unauthorized"))
		return
	}

	var newTheme string
	err := session.Update(userID, func(s *session.Session) {
		if s.Theme == "dark" {
			s.Theme = "light"
		} else {
			s.Theme = "dark"
		}
		newTheme = s.Theme
	})
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(newTheme))
}

func (h *Handler) HandleSetTheme(w http.ResponseWriter, r *http.Request) {
	userID := session.ReadCookie(r)
	if userID == "" {
		h.serverError(w, r, fmt.Errorf("unauthorized"))
		return
	}

	theme := r.URL.Query().Get("theme")
	if theme != "dark" && theme != "light" {
		http.Error(w, "invalid theme", http.StatusBadRequest)
		return
	}

	err := session.Update(userID, func(s *session.Session) {
		s.Theme = theme
	})
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
