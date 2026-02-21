package handlers

import (
	"net/http"

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

	if h.svc.Verification.IsVerified(ctx) {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if isHtmx {
		if err := verifyrecoverypage.Content(user, "").Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifyrecoverypage.Page(state, user, "").Render(ctx, w); err != nil {
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

	user, err := h.svc.Verification.GetCurrentUser(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := h.svc.Verification.RecoverWithKey(ctx, key); err != nil {
		h.logger.Error("recovery key verification failed", "err", err)
		if err := verifyrecoverypage.Content(user, "Invalid recovery key. Please check and try again.").Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}
