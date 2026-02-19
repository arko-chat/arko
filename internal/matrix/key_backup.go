package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
		return "", fmt.Errorf("key backup version: status %d", resp.StatusCode)
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
		return nil, fmt.Errorf("key backup fetch: status %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var data keyBackupSessionData
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

func (m *Manager) tryImportKeyFromBackup(
	ctx context.Context,
	userID string,
	roomID id.RoomID,
	sessionID id.SessionID,
) bool {
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

	m.logger.Info("fetched session from key backup",
		"user", userID,
		"room", roomID,
		"session", sessionID,
	)

	_ = sessData
	return false
}
