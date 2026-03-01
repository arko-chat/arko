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

	props := verifyqrpage.ContentProps{
		User:      user,
		QRCodeSVG: qrSVG,
	}

	if isHtmx {
		if err := verifyqrpage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	h.svc.WebView.SetTitle("QR Verification")

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

	props := verifyqrscannedpage.ContentProps{
		User: user,
	}

	if isHtmx {
		if err := verifyqrscannedpage.Content(props).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}

	h.svc.WebView.SetTitle("QR Verification")

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
