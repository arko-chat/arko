package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/matrix"
	verifywaitingpage "github.com/arko-chat/arko/pages/verify/waiting"
)

func (h *Handler) HandleVerifyWaitingPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()
	isHtmx := htmx.IsHTMX(r)

	verified := h.svc.IsVerified(ctx, state.UserID)
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
	if vState, ok := vs.(*matrix.VerificationUIState); ok && vState != nil {
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

		if len(vState.Emojis) > 0 {
			if isHtmx {
				w.Header().Set("HX-Redirect", "/verify/sas")
				w.WriteHeader(http.StatusOK)
				return
			}
			htmx.Redirect(w, r, "/verify/sas")
			return
		}
	}

	user, err := h.svc.GetCurrentUser(ctx, state.UserID)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if isHtmx {
		if err := verifywaitingpage.Content(user).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifywaitingpage.Page(state, user).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
