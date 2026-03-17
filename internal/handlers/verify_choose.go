package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components"
	"github.com/arko-chat/arko/internal/htmx"
	verifychoosepage "github.com/arko-chat/arko/pages/verify/choose"
)

func (h *Handler) HandleVerifyChoosePage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()

	if h.svc.Verification.IsVerified() {
		h.redirect(w, r, "/")
		return
	}

	if !h.svc.Verification.HasCrossSigningKeys() {
		h.redirect(w, r, "/verify")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := verifychoosepage.ContentProps{
		User: user,
	}

	h.svc.WebView.SetTitle("Verification Options")

	if htmx.IsHTMX(r) {
		if err := verifychoosepage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := verifychoosepage.Page(verifychoosepage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
