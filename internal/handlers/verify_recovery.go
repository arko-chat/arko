package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components"
	"github.com/arko-chat/arko/internal/htmx"
	verifyrecoverypage "github.com/arko-chat/arko/pages/verify/recovery"
)

func (h *Handler) HandleVerifyRecoveryPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()
	isHtmx := htmx.IsHTMX(r)

	if h.svc.Verification.IsVerified() {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := verifyrecoverypage.ContentProps{
		User:   user,
		ErrMsg: "",
	}

	h.svc.WebView.SetTitle("Recovery Key Verification")

	if isHtmx {
		if err := verifyrecoverypage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := verifyrecoverypage.Page(verifyrecoverypage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyRecovery(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.serverError(w, r, err)
		return
	}

	key := r.FormValue("recovery_key")

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := verifyrecoverypage.ContentProps{
		User:   user,
		ErrMsg: "Invalid recovery key. Please check and try again.",
	}

	if err := h.svc.Verification.RecoverWithKey(ctx, key); err != nil {
		h.logger.Error("recovery key verification failed", "err", err)
		if err := verifyrecoverypage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}
