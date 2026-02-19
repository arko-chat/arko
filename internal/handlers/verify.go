package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/pages"
	"github.com/arko-chat/arko/internal/matrix"
)

func (h *Handler) HandleVerifyPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)
	ctx := r.Context()
	htmx := h.isHTMX(r)

	verified := h.svc.IsVerified(state.UserID)
	if verified {
		if htmx {
			w.Header().Set("HX-Redirect", "/")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	user, err := h.svc.GetCurrentUser(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	crossSigningDone := h.svc.HasCrossSigningKeys(state.UserID)

	if crossSigningDone {
		vs := h.svc.GetVerificationState(state.UserID)
		if vState, ok := vs.(*matrix.VerificationState); ok && vState != nil {
			if vState.Cancelled {
				if htmx {
					w.Header().Set("HX-Redirect", "/verify")
					w.WriteHeader(http.StatusOK)
					return
				}
				h.svc.ClearVerificationState(state.UserID)
				if err := pages.VerifyPage(user).Render(ctx, w); err != nil {
					h.serverError(w, r, err)
				}
				return
			}

			if len(vState.Emojis) > 0 {
				emojis := make([]pages.EmojiItem, len(vState.Emojis))
				for i, e := range vState.Emojis {
					emojis[i] = pages.EmojiItem{
						Emoji:       e.Emoji,
						Description: e.Description,
					}
				}
				if htmx {
					if err := pages.VerifyEmojiPageContent(user, emojis).Render(ctx, w); err != nil {
						h.serverError(w, r, err)
					}
					return
				}
				if err := pages.VerifyEmojiPage(user, emojis).Render(ctx, w); err != nil {
					h.serverError(w, r, err)
				}
				return
			}
		}

		if htmx {
			if err := pages.VerifyWaitPageContent(user).Render(ctx, w); err != nil {
				h.serverError(w, r, err)
			}
			return
		}
		if err := pages.VerifyWaitPage(user).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := pages.VerifyPage(user).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifySubmit(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = pages.VerifyAlert("Invalid form data.").
			Render(ctx, w)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = pages.VerifyAlert("Password is required.").
			Render(ctx, w)
		return
	}

	err := h.svc.SetupCrossSigning(ctx, state.UserID, password)
	if err != nil {
		h.logger.Error("cross-signing setup failed",
			"user", state.UserID,
			"err", err,
		)
		w.WriteHeader(http.StatusInternalServerError)
		_ = pages.VerifyAlert(
			"Verification failed. Check your password and try again.",
		).Render(ctx, w)
		return
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleVerifyConfirm(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.state(r)
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
	state := h.state(r)
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
