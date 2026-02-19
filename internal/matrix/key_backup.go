package matrix

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/crypto/hkdf"
	"maunium.net/go/mautrix/crypto/backup"
	"maunium.net/go/mautrix/id"
)

type keyBackupVersionResponse struct {
	Version   string `json:"version"`
	Algorithm string `json:"algorithm"`
}

type keyBackupSessionData struct {
	FirstMessageIndex int             `json:"first_message_index"`
	ForwardedCount    int             `json:"forwarded_count"`
	IsVerified        bool            `json:"is_verified"`
	SessionData       json.RawMessage `json:"session_data"`
}

type encryptedSessionData struct {
	Ciphertext string `json:"ciphertext"`
	Ephemeral  string `json:"ephemeral"`
	MAC        string `json:"mac"`
}

func (m *Manager) fetchKeyBackupVersion(
	ctx context.Context,
	accessToken string,
	hsURL string,
) (string, error) {
	endpoint := strings.TrimRight(hsURL, "/") +
		"/_matrix/client/v3/room_keys/version"

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, endpoint, nil,
	)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf(
			"key backup version: status %d", resp.StatusCode,
		)
	}

	body, _ := io.ReadAll(resp.Body)
	var ver keyBackupVersionResponse
	if err := json.Unmarshal(body, &ver); err != nil {
		return "", err
	}
	return ver.Version, nil
}

func (m *Manager) fetchKeyBackupForRoom(
	ctx context.Context,
	accessToken string,
	hsURL string,
	version string,
	roomID id.RoomID,
	sessionID id.SessionID,
) (*keyBackupSessionData, error) {
	endpoint := fmt.Sprintf(
		"%s/_matrix/client/v3/room_keys/keys/%s/%s?version=%s",
		strings.TrimRight(hsURL, "/"),
		roomID,
		sessionID,
		version,
	)

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, endpoint, nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"key backup fetch: status %d", resp.StatusCode,
		)
	}

	body, _ := io.ReadAll(resp.Body)
	var data keyBackupSessionData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func decodeRecoveryKey(recoveryKey string) ([]byte, error) {
	recoveryKey = strings.ReplaceAll(recoveryKey, " ", "")

	raw, err := base64.RawStdEncoding.DecodeString(recoveryKey)
	if err != nil {
		raw, err = base64.StdEncoding.DecodeString(recoveryKey)
		if err != nil {
			return decodeRecoveryKeyBase58(recoveryKey)
		}
	}

	if len(raw) == 32 {
		return raw, nil
	}

	if len(raw) == 35 && raw[0] == 0x8B && raw[1] == 0x01 {
		keyBytes := raw[2:34]
		var parity byte
		for _, b := range raw[:34] {
			parity ^= b
		}
		if parity != raw[34] {
			return nil, fmt.Errorf("recovery key parity check failed")
		}
		return keyBytes, nil
	}

	return raw, nil
}

func decodeRecoveryKeyBase58(input string) ([]byte, error) {
	const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

	result := make([]byte, 0, 35)
	for _, c := range input {
		idx := strings.IndexRune(alphabet, c)
		if idx < 0 {
			continue
		}
		carry := idx
		for i := len(result) - 1; i >= 0; i-- {
			carry += 58 * int(result[i])
			result[i] = byte(carry % 256)
			carry /= 256
		}
		for carry > 0 {
			result = append([]byte{byte(carry % 256)}, result...)
			carry /= 256
		}
	}

	if len(result) < 35 {
		return nil, fmt.Errorf("recovery key too short after base58 decode")
	}

	if result[0] != 0x8B || result[1] != 0x01 {
		return nil, fmt.Errorf("invalid recovery key prefix")
	}

	keyBytes := result[2:34]

	var parity byte
	for _, b := range result[:34] {
		parity ^= b
	}
	if parity != result[34] {
		return nil, fmt.Errorf("recovery key parity check failed")
	}

	return keyBytes, nil
}

func decryptBackupSessionData(
	recoveryKeyBytes []byte,
	encrypted *encryptedSessionData,
) ([]byte, error) {
	ephemeralPubBytes, err := base64.RawStdEncoding.DecodeString(
		strings.TrimRight(encrypted.Ephemeral, "="),
	)
	if err != nil {
		return nil, fmt.Errorf("decode ephemeral key: %w", err)
	}

	curve := ecdh.X25519()
	privKey, err := curve.NewPrivateKey(recoveryKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("create private key: %w", err)
	}

	ephemeralPub, err := curve.NewPublicKey(ephemeralPubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse ephemeral public key: %w", err)
	}

	sharedSecret, err := privKey.ECDH(ephemeralPub)
	if err != nil {
		return nil, fmt.Errorf("ECDH: %w", err)
	}

	hkdfReader := hkdf.New(sha256.New, sharedSecret, nil, nil)
	derivedKeys := make([]byte, 80)
	if _, err := io.ReadFull(hkdfReader, derivedKeys); err != nil {
		return nil, fmt.Errorf("HKDF: %w", err)
	}

	aesKey := derivedKeys[:32]
	macKey := derivedKeys[32:64]
	aesIV := derivedKeys[64:80]

	ciphertextBytes, err := base64.RawStdEncoding.DecodeString(
		strings.TrimRight(encrypted.Ciphertext, "="),
	)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}

	macBytes, err := base64.RawStdEncoding.DecodeString(
		strings.TrimRight(encrypted.MAC, "="),
	)
	if err != nil {
		return nil, fmt.Errorf("decode MAC: %w", err)
	}

	h := hmac.New(sha256.New, macKey)
	h.Write(ciphertextBytes)
	expectedMAC := h.Sum(nil)[:8]

	if !hmac.Equal(macBytes, expectedMAC) {
		return nil, fmt.Errorf("MAC verification failed")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}

	stream := cipher.NewCTR(block, aesIV)
	plaintext := make([]byte, len(ciphertextBytes))
	stream.XORKeyStream(plaintext, ciphertextBytes)

	return plaintext, nil
}

func (m *Manager) tryImportKeyFromBackup(
	ctx context.Context,
	userID string,
	roomID id.RoomID,
	sessionID id.SessionID,
) bool {
	recoveryKey := m.GetRecoveryKey(userID)
	if recoveryKey == "" {
		m.logger.Debug("no recovery key available",
			"user", userID,
		)
		return false
	}

	m.mu.RLock()
	client := m.clients[userID]
	helper := m.cryptoHelpers[userID]
	m.mu.RUnlock()

	if client == nil || helper == nil {
		return false
	}

	machine := helper.Machine()
	if machine == nil {
		return false
	}

	hsURL := client.HomeserverURL.String()
	accessToken := client.AccessToken

	version, err := m.fetchKeyBackupVersion(ctx, accessToken, hsURL)
	if err != nil || version == "" {
		m.logger.Debug("no key backup available",
			"user", userID,
			"err", err,
		)
		return false
	}

	sessData, err := m.fetchKeyBackupForRoom(
		ctx, accessToken, hsURL, version, roomID, sessionID,
	)
	if err != nil || sessData == nil {
		m.logger.Debug("key backup fetch failed",
			"user", userID,
			"room", roomID,
			"session", sessionID,
			"err", err,
		)
		return false
	}

	recoveryKeyBytes, err := decodeRecoveryKey(recoveryKey)
	if err != nil {
		m.logger.Warn("failed to decode recovery key",
			"user", userID,
			"err", err,
		)
		return false
	}

	var encrypted encryptedSessionData
	if err := json.Unmarshal(sessData.SessionData, &encrypted); err != nil {
		m.logger.Warn("failed to parse encrypted session data",
			"user", userID,
			"err", err,
		)
		return false
	}

	plaintext, err := decryptBackupSessionData(recoveryKeyBytes, &encrypted)
	if err != nil {
		m.logger.Warn("failed to decrypt key backup",
			"user", userID,
			"room", roomID,
			"session", sessionID,
			"err", err,
		)
		return false
	}

	var megolmData backup.MegolmSessionData
	if err := json.Unmarshal(plaintext, &megolmData); err != nil {
		m.logger.Warn("failed to parse decrypted session",
			"user", userID,
			"err", err,
		)
		return false
	}

	_, err = machine.ImportRoomKeyFromBackup(
		ctx,
		id.KeyBackupVersion(version),
		roomID,
		sessionID,
		&megolmData,
	)
	if err != nil {
		m.logger.Warn("failed to import session from backup",
			"user", userID,
			"room", roomID,
			"session", sessionID,
			"err", err,
		)
		return false
	}

	m.logger.Info("imported session from key backup",
		"user", userID,
		"room", roomID,
		"session", sessionID,
	)

	return true
}
