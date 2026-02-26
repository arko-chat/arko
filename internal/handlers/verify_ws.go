package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/arko-chat/arko/internal/ws"
	verifyws "github.com/arko-chat/arko/internal/ws/verify"
)

func (h *Handler) HandleVerifyWS(w http.ResponseWriter, r *http.Request) {
	state := h.session(r)
	userID := state.UserID

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.serverError(w, r, err)
		return
	}

	hub := h.svc.Verification.GetWSHub()
	client := &verifyws.Client{
		Hub:        hub,
		UserID:     userID,
		BaseClient: ws.NewBaseClient(conn),
	}

	hub.Register(userID, client)
	go client.WritePump()

	go h.verifyStatePusher(r.Context(), userID, hub)

	client.ReadPump(func(uid, action string) {
		ctx := context.Background()
		switch action {
		case "SAS_CONFIRM":
			if err := h.svc.Verification.ConfirmVerification(ctx); err != nil {
				h.logger.Error("ws verify confirm failed", "err", err)
			}
		case "SAS_CANCEL":
			if err := h.svc.Verification.CancelVerification(ctx); err != nil {
				h.logger.Error("ws verify cancel failed", "err", err)
			}
			hub.Push(uid, verifyws.RedirectFragment("/verify"))
		case "CONFIRM_QR":
			if err := h.svc.Verification.ConfirmQRVerification(ctx); err != nil {
				h.logger.Error("ws verify qr confirm failed", "err", err)
			}
		}
	})
}

func (h *Handler) verifyStatePusher(ctx context.Context, userID string, hub ws.WSHub) {
	// TODO: replace ticker with a real event subscription from h.svc.Verification
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.pushVerifyState(userID, hub)
		}
	}
}

func (h *Handler) pushVerifyState(userID string, hub ws.WSHub) {
	if h.svc.Verification.IsVerified() {
		hub.Push(userID, verifyws.RedirectFragment("/"))
		return
	}

	vs := h.svc.Verification.GetVerificationState()

	if vs != nil && vs.Cancelled {
		h.svc.Verification.ClearVerificationState()
		hub.Push(userID, verifyws.RedirectFragment("/verify/choose"))
		return
	}

	if !h.svc.Verification.HasCrossSigningKeys() {
		return
	}

	if vs == nil {
		return
	}

	if len(vs.Emojis) > 0 {
		hub.Push(userID, verifyws.RedirectFragment("/verify/sas"))
		return
	}

	if vs.SASActive {
		hub.Push(userID, verifyws.RedirectFragment("/verify/sas/waiting"))
		return
	}

	if vs.QRScanned {
		hub.Push(userID, verifyws.RedirectFragment("/verify/qr/scanned"))
		return
	}
}
