package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/arko-chat/arko/internal/session"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/backup"
	"maunium.net/go/mautrix/crypto/ssss"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type KeyBackupManager struct {
	matrixSession *MatrixSession

	keyMutex   sync.Mutex
	currentKey *backup.MegolmBackupKey
}

func NewKeyBackupManager(
	session *MatrixSession,
) *KeyBackupManager {
	return &KeyBackupManager{
		matrixSession: session,
	}
}

func (kb *KeyBackupManager) Init(ctx context.Context, userID string) error {
	s, err := session.UpdateAndGet(userID, func(s *session.Session) {
		if s.RecoveryKey != "" {
			return
		}

		key, err := kb.matrixSession.ssssMachine.GenerateAndUploadKey(ctx, "")
		if err != nil {
			return
		}
		s.RecoveryKey = key.RecoveryKey()

		kb.matrixSession.ssssMachine.SetDefaultKeyID(ctx, key.ID)

		backupKey, err := kb.ssssKeyToBackupKey(ctx, key)
		if err != nil {
			return
		}

		kb.setCurrentKey(backupKey)
	})
	if err != nil {
		return err
	}

	if kb.currentKey == nil && s.RecoveryKey != "" {
		key, err := kb.GetBackupKeyFromRecoveryKey(ctx, s.RecoveryKey)
		if err != nil {
			return err
		}

		kb.setCurrentKey(key)
	}

	if kb.currentKey != nil {
		return nil
	}

	return fmt.Errorf("unable to acquire backup key")
}

func (kb *KeyBackupManager) ssssKeyToBackupKey(ctx context.Context, ssssKey *ssss.Key) (*backup.MegolmBackupKey, error) {
	var content ssss.EncryptedAccountDataEventContent
	err := kb.matrixSession.GetClient().GetAccountData(ctx, event.AccountDataMegolmBackupKey.Type, &content)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup account data: %w", err)
	}

	_, encryptedForThisKey := content.Encrypted[ssssKey.ID]
	if !encryptedForThisKey {
		newBackupKey, err := backup.NewMegolmBackupKey()
		if err != nil {
			return nil, err
		}

		err = kb.matrixSession.ssssMachine.SetEncryptedAccountData(
			ctx,
			event.AccountDataMegolmBackupKey,
			newBackupKey.Bytes(),
			ssssKey,
		)
		if err != nil {
			return nil, err
		}

		return newBackupKey, nil
	}

	backupKeyBytes, err := kb.matrixSession.ssssMachine.GetDecryptedAccountData(
		ctx,
		event.AccountDataMegolmBackupKey,
		ssssKey,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt megolm backup key: %w", err)
	}

	megolmBackupKey, err := backup.MegolmBackupKeyFromBytes(backupKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse megolm backup key: %w", err)
	}

	return megolmBackupKey, nil
}

func (kb *KeyBackupManager) setCurrentKey(currentKey *backup.MegolmBackupKey) {
	kb.keyMutex.Lock()
	defer kb.keyMutex.Unlock()

	kb.currentKey = currentKey
}

func (kb *KeyBackupManager) GetBackupKeyFromRecoveryKey(
	ctx context.Context,
	recoveryKey string,
) (*backup.MegolmBackupKey, error) {
	keyID, keyData, err := kb.matrixSession.ssssMachine.GetDefaultKeyData(ctx)
	if err != nil {
		return nil, fmt.Errorf("get default SSSS key data: %w", err)
	}

	ssssKey, err := keyData.VerifyRecoveryKey(keyID, recoveryKey)
	if err != nil {
		return nil, fmt.Errorf("verify recovery key: %w", err)
	}

	return kb.ssssKeyToBackupKey(ctx, ssssKey)
}

func (kb *KeyBackupManager) GetBackupKeyFromPassphrase(
	ctx context.Context,
	passphrase string,
) (*backup.MegolmBackupKey, error) {
	keyID, keyData, err := kb.matrixSession.ssssMachine.GetDefaultKeyData(ctx)
	if err != nil {
		return nil, fmt.Errorf("get default SSSS key data: %w", err)
	}

	ssssKey, err := keyData.VerifyPassphrase(keyID, passphrase)
	if err != nil {
		return nil, fmt.Errorf("verify passphrase: %w", err)
	}

	backupKeyBytes, err := kb.matrixSession.ssssMachine.GetDecryptedAccountData(
		ctx,
		event.AccountDataMegolmBackupKey,
		ssssKey,
	)
	if err != nil {
		return nil, fmt.Errorf("decrypt megolm backup key: %w", err)
	}

	megolmBackupKey, err := backup.MegolmBackupKeyFromBytes(backupKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("parse megolm backup key: %w", err)
	}

	return megolmBackupKey, nil
}

func (kb *KeyBackupManager) RestoreRoomKeys(
	ctx context.Context,
	roomID id.RoomID,
) (int, error) {
	if kb.currentKey == nil {
		return 0, fmt.Errorf("backup key has not been initialized yet")
	}

	versionResp, err := kb.matrixSession.GetClient().GetKeyBackupLatestVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("get backup version: %w", err)
	}

	roomKeys, err := kb.matrixSession.GetClient().GetKeyBackupForRoom(
		ctx,
		versionResp.Version,
		roomID,
	)
	if err != nil {
		return 0, fmt.Errorf("get room keys: %w", err)
	}

	cryptoMachine := kb.matrixSession.GetCryptoHelper().Machine()
	imported := 0

	for sessionID, keyData := range roomKeys.Sessions {
		sessionData, err := keyData.SessionData.Decrypt(kb.currentKey)
		if err != nil {
			continue
		}

		existing, err := cryptoMachine.CryptoStore.GetGroupSession(ctx, roomID, sessionID)
		if err != nil {
			return imported, fmt.Errorf("failed to check existing session: %w", err)
		}
		if existing != nil && existing.Internal.FirstKnownIndex() <= uint32(keyData.FirstMessageIndex) {
			continue
		}

		igs, err := crypto.NewInboundGroupSession(
			sessionData.SenderKey,
			sessionData.SenderClaimedKeys.Ed25519,
			roomID,
			sessionData.SessionKey,
			0,
			0,
			false,
		)
		if err != nil {
			continue
		}

		igs.ForwardingChains = sessionData.ForwardingKeyChain
		igs.KeyBackupVersion = versionResp.Version

		err = cryptoMachine.CryptoStore.PutGroupSession(ctx, igs)
		if err != nil {
			continue
		}
		imported++
	}

	return imported, nil
}

func (kb *KeyBackupManager) GetIsVerified(ctx context.Context, userID id.UserID, senderKey id.SenderKey) bool {
	machine := kb.matrixSession.GetCryptoHelper().Machine()

	devices, err := machine.CryptoStore.GetDevices(ctx, userID)
	if err != nil || devices == nil {
		return false
	}

	for _, device := range devices {
		if device.IdentityKey == senderKey {
			return device.Trust == id.TrustStateVerified
		}
	}

	return false
}

func (kb *KeyBackupManager) BackupRoomKeys(ctx context.Context, roomID id.RoomID, userID id.UserID, sessionID id.SessionID) error {
	if kb.currentKey == nil {
		return fmt.Errorf("backup key not initialized")
	}

	machine := kb.matrixSession.cryptoHelper.Machine()
	igs, err := machine.CryptoStore.GetGroupSession(ctx, roomID, sessionID)
	if err != nil {
		return err
	}

	versionResp, err := kb.matrixSession.GetClient().GetKeyBackupLatestVersion(ctx)
	if err != nil {
		return err
	}

	firstIndex := igs.Internal.FirstKnownIndex()

	exportedKey, err := igs.Internal.Export(firstIndex)
	if err != nil {
		return fmt.Errorf("failed to export session: %w", err)
	}

	sessionData := &backup.MegolmSessionData{
		Algorithm:          id.AlgorithmMegolmV1,
		SenderKey:          igs.SenderKey,
		SenderClaimedKeys:  backup.SenderClaimedKeys{Ed25519: igs.SigningKey},
		ForwardingKeyChain: igs.ForwardingChains,
		SessionKey:         string(exportedKey),
	}

	encryptedData, err := backup.EncryptSessionData(kb.currentKey, sessionData)
	if err != nil {
		return fmt.Errorf("failed to encrypt session data: %w", err)
	}

	marshaledData, err := json.Marshal(encryptedData)
	if err != nil {
		return fmt.Errorf("failed to marshal encrypted data: %w", err)
	}

	req := &mautrix.ReqKeyBackupData{
		FirstMessageIndex: int(firstIndex),
		ForwardedCount:    len(igs.ForwardingChains),
		IsVerified:        kb.GetIsVerified(ctx, userID, igs.SenderKey),
		SessionData:       marshaledData,
	}

	_, err = kb.matrixSession.GetClient().PutKeysInBackupForRoomAndSession(
		ctx,
		versionResp.Version,
		roomID,
		igs.ID(),
		req,
	)
	if err != nil {
		return fmt.Errorf("failed to upload to key backup: %w", err)
	}

	return nil
}
