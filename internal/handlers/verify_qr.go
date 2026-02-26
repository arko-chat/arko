package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	verifyqrpage "github.com/arko-chat/arko/pages/verify/qr"
	verifyqrscannedpage "github.com/arko-chat/arko/pages/verify/qr/scanned"
)

func (h *Handler) HandleVerifyStartQR(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := h.svc.Verification.RequestQRVerification(ctx); err != nil {
		h.logger.Error("failed to start QR verification", "err", err)
		h.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/verify/qr")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleVerifyQRPage(
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

	if !h.svc.Verification.HasCrossSigningKeys() {
		redirect("/verify")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		redirect("/verify/choose")
		return
	}

	if vs.QRScanned {
		redirect("/verify/qr/scanned")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	qrSVG, err := h.svc.Verification.GetQRCodeSVG(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if isHtmx {
		if err := verifyqrpage.Content(user, qrSVG).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifyqrpage.Page(state, user, qrSVG).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyQRScannedPage(
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

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		redirect("/verify/choose")
		return
	}

	if !vs.QRScanned {
		redirect("/verify/qr")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if isHtmx {
		if err := verifyqrscannedpage.Content(user).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifyqrscannedpage.Page(state, user).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyQRStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	redirect := func(path string) {
		w.Header().Set("HX-Redirect", path)
		w.WriteHeader(http.StatusOK)
	}

	if h.svc.Verification.IsVerified() {
		redirect("/")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		redirect("/verify/choose")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if vs.QRScanned {
		if err := verifyqrscannedpage.Content(user).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	qrSVG, err := h.svc.Verification.GetQRCodeSVG(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if err := verifyqrpage.Content(user, qrSVG).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
