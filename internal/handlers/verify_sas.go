package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	verifysaspage "github.com/arko-chat/arko/pages/verify/sas"
)

func (h *Handler) HandleVerifySASPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()
	isHtmx := htmx.IsHTMX(r)

	verified := h.svc.Verification.IsVerified(ctx)
	if verified {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/")
		return
	}

	if !h.svc.Verification.HasCrossSigningKeys() {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs == nil || len(vs.Emojis) == 0 {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify/waiting")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify/waiting")
		return
	}

	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser(ctx)
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

	if isHtmx {
		if err := verifysaspage.Content(user, emojis).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifysaspage.Page(state, user, emojis).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyConfirm(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := h.svc.Verification.ConfirmVerification(ctx); err != nil {
		h.logger.Error("verification confirm failed",
			"user",
			"err", err,
		)
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleVerifyCancel(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := h.svc.Verification.CancelVerification(ctx); err != nil {
		h.logger.Error("verification cancel failed",
			"user",
			"err", err,
		)
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}
