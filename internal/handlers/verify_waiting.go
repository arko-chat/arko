package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	verifywaitingpage "github.com/arko-chat/arko/pages/verify/waiting"
)

func (h *Handler) HandleVerifyWaitingPage(
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

	if len(vs.Emojis) > 0 {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify/sas")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify/sas")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser(ctx)
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
