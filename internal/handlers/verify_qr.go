package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/components"
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

	h.htmxRedirect(w, "/verify/qr")
}

func (h *Handler) HandleVerifyQRPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()

	if h.svc.Verification.IsVerified() {
		h.redirect(w, r, "/")
		return
	}

	if !h.svc.Verification.HasCrossSigningKeys() {
		h.redirect(w, r, "/verify")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		h.redirect(w, r, "/verify/choose")
		return
	}

	if vs.QRScanned {
		h.redirect(w, r, "/verify/qr/scanned")
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

	props := verifyqrpage.ContentProps{
		User:      user,
		QRCodeSVG: qrSVG,
	}

	h.svc.WebView.SetTitle("QR Verification")

	if htmx.IsHTMX(r) {
		if err := verifyqrpage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := verifyqrpage.Page(verifyqrpage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyQRScannedPage(
	w http.ResponseWriter,
	r *http.Request,
) {
	state := h.session(r)
	ctx := r.Context()

	if h.svc.Verification.IsVerified() {
		h.redirect(w, r, "/")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		h.redirect(w, r, "/verify/choose")
		return
	}

	if !vs.QRScanned {
		h.redirect(w, r, "/verify/qr")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := verifyqrscannedpage.ContentProps{
		User: user,
	}

	h.svc.WebView.SetTitle("QR Verification")

	if htmx.IsHTMX(r) {
		if err := verifyqrscannedpage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	if err := verifyqrscannedpage.Page(verifyqrscannedpage.PageProps{
		PageProps: components.PageProps{
			State: state,
			Title: h.svc.WebView.GetTitle(),
		},
		ContentProps: props,
	}).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyQRStatus(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if h.svc.Verification.IsVerified() {
		h.htmxRedirect(w, "/")
		return
	}

	vs := h.svc.Verification.GetVerificationState()
	if vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		h.htmxRedirect(w, "/verify/choose")
		return
	}

	user, err := h.svc.Verification.GetCurrentUser()
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	propsScanned := verifyqrscannedpage.ContentProps{
		User: user,
	}

	h.svc.WebView.SetTitle("QR Verification")

	if vs.QRScanned {
		if err := verifyqrscannedpage.Content(propsScanned).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	qrSVG, err := h.svc.Verification.GetQRCodeSVG(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	props := verifyqrpage.ContentProps{
		User:      user,
		QRCodeSVG: qrSVG,
	}

	if err := verifyqrpage.Content(props).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
