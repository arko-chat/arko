package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	verifywaitingpage "github.com/arko-chat/arko/pages/verify/waiting"
)

func (h *Handler) HandleVerifyPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	isHtmx := htmx.IsHTMX(r)

	redirect := func(path string) {
		if isHtmx {
			w.Header().Set("HX-Redirect", path)
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, path)
	}

	if h.svc.Verification.IsVerified() {
		redirect("/")
		return
	}

	if !h.svc.Verification.HasCrossSigningKeys() {
		redirect("/verify/waiting")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		redirect("/verify/choose")
		return
	}

	if len(vs.Emojis) > 0 {
		redirect("/verify/sas")
		return
	}

	if vs.SASActive {
		redirect("/verify/sas/waiting")
		return
	}

	if vs.QRScanned {
		redirect("/verify/qr/scanned")
		return
	}

	if vs.QRActive {
		redirect("/verify/qr")
		return
	}

	redirect("/verify/choose")
}

func (h *Handler) HandleVerifyWaitingPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()
	isHtmx := htmx.IsHTMX(r)

	redirect := func(path string) {
		if isHtmx {
			w.Header().Set("HX-Redirect", path)
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, path)
	}

	if h.svc.Verification.IsVerified() {
		redirect("/")
		return
	}

	if h.svc.Verification.HasCrossSigningKeys() {
		redirect("/verify/choose")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
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
