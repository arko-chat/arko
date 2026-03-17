package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components"
	"github.com/arko-chat/arko/internal/htmx"
	verifysaspage "github.com/arko-chat/arko/pages/verify/sas"
	verifysaswaitingpage "github.com/arko-chat/arko/pages/verify/sas/waiting"
)

func (h *Handler) HandleVerifyStartSAS(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := h.svc.Verification.RequestSASVerification(ctx); err != nil {
		h.logger.Error("failed to start SAS verification", "err", err)
		h.serverError(w, r, err)
		return
	}

	h.htmxRedirect(w, "/verify/sas/waiting")
}

func (h *Handler) HandleVerifySASPage(
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

	vs := h.svc.Verification.GetVerificationState()
	if vs == nil || len(vs.Emojis) == 0 {
		h.redirect(w, r, "/verify/sas/waiting")
		return
	}

	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		h.redirect(w, r, "/verify")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	emojis := make([]verifysaspage.EmojiItem, len(vs.Emojis))
	for i, e := range vs.Emojis {
		emojis[i] = verifysaspage.EmojiItem{
			Emoji:       e.Emoji,
			Description: e.Description,
		}
	}

	props := verifysaspage.ContentProps{
		User:   user,
		Emojis: emojis,
	}

	h.svc.WebView.SetTitle("SAS Verification")

	if htmx.IsHTMX(r) {
		if err := verifysaspage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := verifysaspage.Page(verifysaspage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifySASWaitingPage(
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
		h.redirect(w, r, "/verify/waiting")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		h.redirect(w, r, "/verify/choose")
		return
	}

	if len(vs.Emojis) > 0 {
		h.redirect(w, r, "/verify/sas")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := verifysaswaitingpage.ContentProps{
		User: user,
	}

	h.svc.WebView.SetTitle("SAS Verification")

	if htmx.IsHTMX(r) {
		if err := verifysaswaitingpage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := verifysaswaitingpage.Page(verifysaswaitingpage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
