package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/matrix"
	verifysaspage "github.com/arko-chat/arko/pages/verify/sas"
)

func (h *Handler) HandleVerifySASPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()
	isHtmx := htmx.IsHTMX(r)

	verified := h.svc.IsVerified(state.UserID)
	if verified {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/")
		return
	}

	if !h.svc.HasCrossSigningKeys(state.UserID) {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify")
		return
	}

	vs := h.svc.GetVerificationState(state.UserID)
	vState, ok := vs.(*matrix.VerificationState)
	if !ok || vState == nil || len(vState.Emojis) == 0 {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify/waiting")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify/waiting")
		return
	}

	if vState.Cancelled {
		h.svc.ClearVerificationState(state.UserID)
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify")
		return
	}

	user, err := h.svc.GetCurrentUser(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	emojis := make([]verifysaspage.EmojiItem, len(vState.Emojis))
	for i, e := range vState.Emojis {
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
	state := h.session(r)
	ctx := r.Context()

	if err := h.svc.ConfirmVerification(ctx, state.UserID); err != nil {
		h.logger.Error("verification confirm failed",
			"user", state.UserID,
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
	state := h.session(r)
	ctx := r.Context()

	if err := h.svc.CancelVerification(ctx, state.UserID); err != nil {
		h.logger.Error("verification cancel failed",
			"user", state.UserID,
			"err", err,
		)
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}
