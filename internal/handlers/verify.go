package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components/ui"
	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/matrix"
	verifypage "github.com/arko-chat/arko/pages/verify"
)

func (h *Handler) HandleVerifyPage(
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

	crossSigningDone := h.svc.HasCrossSigningKeys(state.UserID)

	if crossSigningDone {
		vs := h.svc.GetVerificationState(state.UserID)
		if vState, ok := vs.(*matrix.VerificationState); ok && vState != nil {
			if vState.Cancelled {
				h.svc.ClearVerificationState(state.UserID)
			} else if len(vState.Emojis) > 0 {
				if isHtmx {
					w.Header().Set("HX-Redirect", "/verify/sas")
					w.WriteHeader(http.StatusOK)
					return
				}
				htmx.Redirect(w, r, "/verify/sas")
				return
			}
		}

		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify/waiting")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify/waiting")
		return
	}

	user, err := h.svc.GetCurrentUser(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := verifypage.Page(state, user).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifySubmit(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = ui.Alert("Invalid form data.").
			Render(ctx, w)
		return
	}

	password := r.FormValue("password")
	if password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = ui.Alert("Password is required.").
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
		_ = ui.Alert(
			"Verification failed. Check your password and try again.",
		).Render(ctx, w)
		return
	}

	w.Header().Set("HX-Redirect", "/verify/waiting")
	w.WriteHeader(http.StatusOK)
}
