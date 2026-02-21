package handlers

import (
	"net/http"

	"github.com/arko-chat/arko/internal/htmx"
	verifysaspage "github.com/arko-chat/arko/pages/verify/sas"
	verifysaswaitingpage "github.com/arko-chat/arko/pages/verify/sas/waiting"
)

func (h *Handler) HandleVerifyStartSAS(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := h.svc.Verification.RequestSASVerification(ctx); err != nil {
		h.logger.Error("failed to start SAS verification", "err", err)
		h.serverError(w, r, err)
		return
	}

	w.Header().Set("HX-Redirect", "/verify/sas/waiting")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleVerifySASPage(
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
	if vs == nil || len(vs.Emojis) == 0 {
		if isHtmx {
			w.Header().Set("HX-Redirect", "/verify/sas/waiting")
			w.WriteHeader(http.StatusOK)
			return
		}
		htmx.Redirect(w, r, "/verify/sas/waiting")
		return
	}

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

	user, err := h.svc.Verification.GetCurrentUser(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	emojis := make([]verifysaspage.EmojiItem, len(vs.Emojis))
	for i, e := range vs.Emojis {
		emojis[i] = verifysaspage.EmojiItem{
			Emoji:       e.Emoji,
			Description: e.Description,
		}
	}

	if isHtmx {
		if err := verifysaspage.Content(user, emojis).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifysaspage.Page(state, user, emojis).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}

func (h *Handler) HandleVerifyConfirm(
	w http.ResponseWriter,
	r *http.Request,
) {
	ctx := r.Context()

	if err := h.svc.Verification.ConfirmVerification(ctx); err != nil {
		h.logger.Error("verification confirm failed",
			"user",
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
	ctx := r.Context()

	if err := h.svc.Verification.CancelVerification(ctx); err != nil {
		h.logger.Error("verification cancel failed",
			"user",
			"err", err,
		)
	}

	w.Header().Set("HX-Redirect", "/verify")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleVerifySASWaitingPage(
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

	if h.svc.Verification.IsVerified(ctx) {
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

	user, err := h.svc.Verification.GetCurrentUser(ctx)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	if isHtmx {
		if err := verifysaswaitingpage.Content(user).Render(ctx, w); err != nil {
			h.serverError(w, r, err)
		}
		return
	}
	if err := verifysaswaitingpage.Page(state, user).Render(ctx, w); err != nil {
		h.serverError(w, r, err)
	}
}
