package credentials

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const (
	serviceName    = "arko"
	keyAccessToken = "access_token"
	keyRecoveryKey = "recovery_key"
	keyVerified    = "verified"
)

var ErrNotFound = errors.New("credentials: not found")

type SessionMetadata struct {
	Homeserver string `json:"homeserver"`
	UserID     string `json:"user_id"`
	DeviceID   string `json:"device_id"`
}

func StoreSession(meta SessionMetadata, accessToken string) error {
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal session metadata: %w", err)
	}

	if err := keyring.Set(serviceName, meta.UserID+":metadata", string(metaJSON)); err != nil {
		return fmt.Errorf("store metadata: %w", err)
	}

	if err := keyring.Set(serviceName, meta.UserID+":"+keyAccessToken, accessToken); err != nil {
		return fmt.Errorf("store access token: %w", err)
	}

	return nil
}

func LoadSession(userID string) (SessionMetadata, string, error) {
	metaRaw, err := keyring.Get(serviceName, userID+":metadata")
	if err != nil {
		return SessionMetadata{}, "", ErrNotFound
	}

	var meta SessionMetadata
	if err := json.Unmarshal([]byte(metaRaw), &meta); err != nil {
		return SessionMetadata{}, "", fmt.Errorf("unmarshal metadata: %w", err)
	}

	token, err := keyring.Get(serviceName, userID+":"+keyAccessToken)
	if err != nil {
		return SessionMetadata{}, "", fmt.Errorf("load access token: %w", err)
	}

	return meta, token, nil
}

func DeleteSession(userID string) {
	_ = keyring.Delete(serviceName, userID+":metadata")
	_ = keyring.Delete(serviceName, userID+":"+keyAccessToken)
}

func StoreAppSecret(key string, value string) error {
	return keyring.Set(serviceName, "app:"+key, value)
}

func LoadAppSecret(key string) (string, error) {
	val, err := keyring.Get(serviceName, "app:"+key)
	if err != nil {
		return "", ErrNotFound
	}
	return val, nil
}

func DeleteAppSecret(key string) {
	_ = keyring.Delete(serviceName, "app:"+key)
}

func StoreRecoveryKey(userID string, key string) error {
	return keyring.Set(serviceName, userID+":"+keyRecoveryKey, key)
}

func LoadRecoveryKey(userID string) (string, error) {
	val, err := keyring.Get(serviceName, userID+":"+keyRecoveryKey)
	if err != nil {
		return "", ErrNotFound
	}
	return val, nil
}

func DeleteRecoveryKey(userID string) {
	_ = keyring.Delete(serviceName, userID+":"+keyRecoveryKey)
}

func StoreVerified(userID string, verified bool) error {
	val := "false"
	if verified {
		val = "true"
	}
	return keyring.Set(serviceName, userID+":"+keyVerified, val)
}

func LoadVerified(userID string) bool {
	val, err := keyring.Get(serviceName, userID+":"+keyVerified)
	if err != nil {
		return false
	}
	return val == "true"
}

func DeleteVerified(userID string) {
	_ = keyring.Delete(serviceName, userID+":"+keyVerified)
}
