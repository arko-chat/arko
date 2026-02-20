package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	"github.com/arko-chat/arko/internal/matrix"
)

func (h *Handler) HandleVerifyPage(
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

	crossSigningDone := h.svc.HasCrossSigningKeys(state.UserID)
	if crossSigningDone {
		vs := h.svc.GetVerificationState(state.UserID)
		if vState, ok := vs.(*matrix.VerificationUIState); ok && vState != nil {
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
	}

	htmx.Redirect(w, r, "/verify/waiting")
	return
}
