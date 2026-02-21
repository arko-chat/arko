package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
)

func (h *Handler) HandleVerifyPage(
	w http.ResponseWriter,
	r *http.Request,
) {
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

	crossSigningDone := h.svc.Verification.HasCrossSigningKeys()
	if crossSigningDone {
		vs := h.svc.Verification.GetVerificationState()
		if vs.Cancelled {
			h.svc.Verification.ClearVerificationState()
		} else if len(vs.Emojis) > 0 {
			if isHtmx {
				w.Header().Set("HX-Redirect", "/verify/sas")
				w.WriteHeader(http.StatusOK)
				return
			}
			htmx.Redirect(w, r, "/verify/sas")
			return
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
