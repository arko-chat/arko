package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	verifychoosepage "github.com/arko-chat/arko/pages/verify/choose"
)

func (h *Handler) HandleVerifyChoosePage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()
	isHtmx := htmx.IsHTMX(r)

	redirect := func(path string) {
		if isHtmx {
			w.Header().Set("HX-Redirect", path)
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, path)
	}

	if h.svc.Verification.IsVerified() {
		redirect("/")
		return
	}

	if !h.svc.Verification.HasCrossSigningKeys() {
		redirect("/verify")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if isHtmx {
		if err := verifychoosepage.Content(user).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifychoosepage.Page(state, user).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
