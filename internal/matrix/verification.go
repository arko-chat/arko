package matrix

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"strings"

	"encoding/base64"

	"github.com/puzpuzpuz/xsync/v4"
	"github.com/skip2/go-qrcode"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto/verificationhelper"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type VerificationEmoji struct {
	Emoji       string
	Description string
}

type VerificationUIState struct {
	Emojis       []VerificationEmoji
	Decimals     [3]uint
	SASActive    bool
	QRScanned    bool
	QRActive     bool
	QRCode       *verificationhelper.QRCode
	Cancelled    bool
	CancelReason string
	Done         bool
}

func (s *VerificationUIState) Clear() {
	s.Emojis = nil
	s.Decimals = [3]uint{}
	s.SASActive = false
	s.QRActive = false
	s.QRScanned = false
	s.QRCode = nil
	s.Cancelled = false
	s.CancelReason = ""
	s.Done = false
}

func (m *Manager) getActiveTransaction(
	userID string,
) (verificationhelper.VerificationTransaction, error) {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return verificationhelper.VerificationTransaction{},
			fmt.Errorf("no verification store for user")
	}

	txns, err := mSess.GetVerificationStore().GetAllVerificationTransactions(context.Background())
	if err != nil {
		return verificationhelper.VerificationTransaction{},
			fmt.Errorf("get transactions: %w", err)
	}
	if len(txns) == 0 {
		return verificationhelper.VerificationTransaction{},
			fmt.Errorf("no active verification session")
	}

	return txns[0], nil
}

func (m *Manager) RequestSASVerification(
	ctx context.Context,
	userID string,
) error {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return fmt.Errorf("no active session for user")
	}

	txnID, err := mSess.GetVerificationHelper().StartVerification(
		ctx,
		id.UserID(userID),
	)
	if err != nil {
		return fmt.Errorf("start SAS verification: %w", err)
	}

	m.matrixSessions.Compute(userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().SASActive = true
		return oldValue, xsync.UpdateOp
	})

	m.logger.Info("SAS verification started", "user", userID, "txnID", txnID)
	return nil
}

func (m *Manager) RequestQRVerification(
	ctx context.Context,
	userID string,
) error {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return fmt.Errorf("no active session for user")
	}

	txnID, err := mSess.GetVerificationHelper().StartVerification(
		ctx,
		id.UserID(userID),
	)
	if err != nil {
		return fmt.Errorf("start QR verification: %w", err)
	}

	m.matrixSessions.Compute(userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().QRActive = true
		return oldValue, xsync.UpdateOp
	})

	m.logger.Info("QR verification started", "user", userID, "txnID", txnID)
	return nil
}

func (m *Manager) GetQRCodeSVG(
	ctx context.Context,
	userID string,
) (string, error) {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return "", fmt.Errorf("no active session for user")
	}

	vs := mSess.GetVerificationUIState()
	if vs.QRCode == nil {
		return "", fmt.Errorf("no QR code available yet")
	}

	data := vs.QRCode.Bytes()
	qr, err := qrcode.New(string(data), qrcode.High)
	if err != nil {
		return "", fmt.Errorf("generate QR code: %w", err)
	}

	var buf bytes.Buffer
	img := qr.Image(256)
	if err := png.Encode(&buf, img); err != nil {
		return "", fmt.Errorf("encode QR PNG: %w", err)
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return `<img src="data:image/png;base64,` + b64 + `" width="256" height="256" alt="QR Code" class="rounded-lg" />`, nil
}

func (m *Manager) ConfirmVerification(
	ctx context.Context,
	userID string,
) error {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return fmt.Errorf("no active verification session")
	}

	txn, err := m.getActiveTransaction(userID)
	if err != nil {
		return fmt.Errorf("no active verification session")
	}

	return mSess.GetVerificationHelper().ConfirmSAS(ctx, txn.TransactionID)
}

func (m *Manager) ConfirmQRVerification(
	ctx context.Context,
	userID string,
) error {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return fmt.Errorf("no active verification session")
	}

	txn, err := m.getActiveTransaction(userID)
	if err != nil {
		return fmt.Errorf("no active verification session")
	}

	return mSess.GetVerificationHelper().ConfirmQRCodeScanned(ctx, txn.TransactionID)
}

func (m *Manager) CancelVerification(
	ctx context.Context,
	userID string,
) error {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return fmt.Errorf("no active verification session")
	}

	txn, err := m.getActiveTransaction(userID)
	if err != nil {
		return fmt.Errorf("no active verification session")
	}

	return mSess.GetVerificationHelper().CancelVerification(
		ctx,
		txn.TransactionID,
		event.VerificationCancelCodeUser,
		"User cancelled",
	)
}

func (m *Manager) RecoverWithKey(
	ctx context.Context,
	userID string,
	recoveryKey string,
) error {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return fmt.Errorf("no active session for user")
	}

	machine := mSess.GetCryptoHelper().Machine()
	if machine == nil {
		return fmt.Errorf("no crypto machine available")
	}

	_, key, err := machine.SSSS.GetDefaultKeyData(ctx)
	if err != nil {
		return fmt.Errorf("get SSSS key data: %w", err)
	}

	normalizedKey := strings.ReplaceAll(recoveryKey, " ", "")

	ssssKey, err := key.VerifyRecoveryKey(userID, normalizedKey)
	if err != nil {
		return fmt.Errorf("invalid recovery key: %w", err)
	}

	if err := machine.FetchCrossSigningKeysFromSSSS(ctx, ssssKey); err != nil {
		return fmt.Errorf("fetch cross-signing keys: %w", err)
	}

	m.matrixSessions.Compute(userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().Done = true
		return oldValue, xsync.UpdateOp
	})

	return nil
}

func (m *Manager) HasCrossSigningKeys(userID string) bool {
	mSess, ok := m.matrixSessions.Load(userID)
	if !ok {
		return false
	}

	machine := mSess.GetCryptoHelper().Machine()
	if machine == nil {
		return false
	}

	pubkeys := machine.GetOwnCrossSigningPublicKeys(context.Background())
	return pubkeys != nil
}

func (m *Manager) ClearVerificationState(userID string) {
	m.matrixSessions.Compute(userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().Clear()
		return oldValue, xsync.UpdateOp
	})
}

type verificationCallbacks struct {
	manager *Manager
	userID  string
	client  *mautrix.Client
}

func (c *verificationCallbacks) VerificationRequested(
	ctx context.Context,
	txnID id.VerificationTransactionID,
	from id.UserID,
	fromDevice id.DeviceID,
) {
	mSess, ok := c.manager.matrixSessions.Load(c.userID)
	if !ok {
		return
	}

	c.manager.logger.Info("verification requested",
		"user", c.userID,
		"from", from,
		"fromDevice", fromDevice,
	)

	if err := mSess.GetVerificationHelper().AcceptVerification(ctx, txnID); err != nil {
		c.manager.logger.Error("failed to auto-accept verification",
			"user", c.userID,
			"err", err,
		)
	}
}

func (c *verificationCallbacks) VerificationReady(
	ctx context.Context,
	txnID id.VerificationTransactionID,
	otherDeviceID id.DeviceID,
	supportsSAS bool,
	supportsScanQRCode bool,
	qrCode *verificationhelper.QRCode,
) {
	c.manager.logger.Info("verification ready",
		"user", c.userID,
		"txnID", txnID,
		"otherDevice", otherDeviceID,
		"supportsSAS", supportsSAS,
		"supportsQR", supportsScanQRCode,
	)

	if qrCode != nil {
		c.manager.matrixSessions.Compute(c.userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
			oldValue.GetVerificationUIState().QRCode = qrCode
			oldValue.GetVerificationUIState().QRActive = true
			return oldValue, xsync.UpdateOp
		})
	}
}

func (c *verificationCallbacks) VerificationCancelled(
	ctx context.Context,
	txnID id.VerificationTransactionID,
	code event.VerificationCancelCode,
	reason string,
) {
	c.manager.matrixSessions.Compute(c.userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().Cancelled = true
		oldValue.GetVerificationUIState().CancelReason = reason
		return oldValue, xsync.UpdateOp
	})

	c.manager.logger.Info("verification cancelled",
		"user", c.userID,
		"code", code,
		"reason", reason,
	)
}

func (c *verificationCallbacks) VerificationDone(
	ctx context.Context,
	txnID id.VerificationTransactionID,
	method event.VerificationMethod,
) {
	c.manager.matrixSessions.Compute(c.userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().Done = true
		return oldValue, xsync.UpdateOp
	})

	mSess, ok := c.manager.matrixSessions.Load(c.userID)
	if !ok {
		return
	}

	txn, err := mSess.GetVerificationStore().GetVerificationTransaction(ctx, txnID)
	if err != nil {
		all, _ := mSess.GetVerificationStore().GetAllVerificationTransactions(ctx)
		c.manager.logger.Error("verification transaction missing",
			"user", c.userID,
			"method", method,
			"txnID", txnID,
			"allTxns", all,
			"error", err,
		)
		return
	}

	c.manager.logger.Info("verification done",
		"user", c.userID,
		"method", method,
		"txnID", txnID,
		"state", txn.VerificationState,
		"theirDone", txn.ReceivedTheirDone,
		"theirMAC", txn.ReceivedTheirMAC,
		"ourDone", txn.SentOurDone,
		"ourMAC", txn.SentOurMAC,
		"sentToDeviceIDs", txn.SentToDeviceIDs,
		"theirDeviceID", txn.TheirDeviceID,
		"theirUserID", txn.TheirUserID,
	)

	machine := mSess.GetCryptoHelper().Machine()
	if machine == nil {
		return
	}

	if txn.ReceivedTheirMAC && txn.SentOurMAC {
		device, err := machine.CryptoStore.GetDevice(
			ctx, id.UserID(c.userID), machine.Client.DeviceID,
		)
		if err != nil || device == nil {
			c.manager.logger.Error("failed to get device",
				"user", c.userID,
				"error", err,
			)
			return
		}

		device.Trust = id.TrustStateCrossSignedTOFU
		_ = machine.CryptoStore.PutDevice(ctx, id.UserID(c.userID), device)
		machine.ShareKeys(ctx, -1)
	}
}

func (c *verificationCallbacks) ShowSAS(
	ctx context.Context,
	txnID id.VerificationTransactionID,
	emojis []rune,
	emojiDescriptions []string,
	decimals []int,
) {
	mapped := make([]VerificationEmoji, len(emojis))
	for i, e := range emojis {
		desc := ""
		if i < len(emojiDescriptions) {
			desc = emojiDescriptions[i]
		}
		mapped[i] = VerificationEmoji{
			Emoji:       string(e),
			Description: desc,
		}
	}

	var dec [3]uint
	for i := 0; i < 3 && i < len(decimals); i++ {
		dec[i] = uint(decimals[i])
	}

	c.manager.matrixSessions.Compute(c.userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().Emojis = mapped
		oldValue.GetVerificationUIState().Decimals = dec
		oldValue.GetVerificationUIState().SASActive = true
		return oldValue, xsync.UpdateOp
	})

	c.manager.logger.Info("SAS ready to confirm", "user", c.userID, "txnID", txnID)
}

func (c *verificationCallbacks) QRCodeScanned(
	ctx context.Context,
	txnID id.VerificationTransactionID,
) {
	c.manager.matrixSessions.Compute(c.userID, func(oldValue *MatrixSession, loaded bool) (newValue *MatrixSession, op xsync.ComputeOp) {
		oldValue.GetVerificationUIState().QRScanned = true
		return oldValue, xsync.UpdateOp
	})

	c.manager.logger.Info("QR code scanned by other device", "user", c.userID, "txnID", txnID)
}
