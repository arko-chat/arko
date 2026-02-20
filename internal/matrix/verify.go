package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (m *Manager) SetupCrossSigningInteractive(
	ctx context.Context,
	userID string,
	password string,
) error {
	m.mu.RLock()
	client := m.clients[userID]
	helper := m.cryptoHelpers[userID]
	m.mu.RUnlock()

	if client == nil {
		return ErrNoClient
	}

	if helper == nil {
		dbPath := fmt.Sprintf(
			"%s/%s.db",
			m.cryptoDBPath,
			url.PathEscape(userID),
		)
		if err := m.setupCrypto(
			ctx, userID, dbPath,
		); err != nil {
			return fmt.Errorf("crypto setup: %w", err)
		}

		m.mu.RLock()
		helper = m.cryptoHelpers[userID]
		m.mu.RUnlock()

		if helper == nil {
			return fmt.Errorf(
				"crypto helper unavailable after setup for %s",
				userID,
			)
		}
	}

	machine := helper.Machine()
	if machine == nil {
		return fmt.Errorf("no olm machine available")
	}

	pubKeysCache, err := machine.GenerateCrossSigningKeys()
	if err != nil {
		return fmt.Errorf("generate cross-signing keys: %w", err)
	}

	masterJSON, _ := json.Marshal(pubKeysCache.MasterKey)
	selfJSON, _ := json.Marshal(pubKeysCache.SelfSigningKey)
	userJSON, _ := json.Marshal(pubKeysCache.UserSigningKey)

	var masterRaw, selfRaw, userRaw json.RawMessage
	masterRaw = masterJSON
	selfRaw = selfJSON
	userRaw = userJSON

	hsURL := strings.TrimRight(
		client.HomeserverURL.String(), "/",
	)
	endpoint := hsURL +
		"/_matrix/client/v3/keys/device_signing/upload"

	session, err := m.crossSigningUploadRaw(
		ctx, client.AccessToken, endpoint,
		masterRaw, selfRaw, userRaw, nil,
	)
	if err != nil && session == "" {
		return fmt.Errorf("initial upload: %w", err)
	}

	if session != "" {
		auth := map[string]any{
			"type":    "m.login.password",
			"session": session,
			"identifier": map[string]any{
				"type": "m.id.user",
				"user": userID,
			},
			"password": password,
		}
		authJSON, _ := json.Marshal(auth)
		authRaw := json.RawMessage(authJSON)

		_, err = m.crossSigningUploadRaw(
			ctx, client.AccessToken, endpoint,
			masterRaw, selfRaw, userRaw, &authRaw,
		)
		if err != nil {
			return fmt.Errorf("authenticated upload: %w", err)
		}
	}

	err = machine.PublishCrossSigningKeys(
		ctx, pubKeysCache, nil,
	)
	if err != nil {
		m.logger.Warn("failed to publish cross-signing keys",
			"user", userID,
			"err", err,
		)
	}

	return nil
}

type crossSigningUploadReq struct {
	MasterKey      json.RawMessage  `json:"master_key"`
	SelfSigningKey json.RawMessage  `json:"self_signing_key"`
	UserSigningKey json.RawMessage  `json:"user_signing_key"`
	Auth           *json.RawMessage `json:"auth,omitempty"`
}

func (m *Manager) crossSigningUploadRaw(
	ctx context.Context,
	accessToken string,
	endpoint string,
	master, self, user json.RawMessage,
	auth *json.RawMessage,
) (string, error) {
	body := crossSigningUploadReq{
		MasterKey:      master,
		SelfSigningKey: self,
		UserSigningKey: user,
		Auth:           auth,
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint,
		bytes.NewReader(bodyJSON),
	)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusOK {
		return "", nil
	}

	var respData struct {
		ErrCode string `json:"errcode"`
		Session string `json:"session"`
		Flows   []struct {
			Stages []string `json:"stages"`
		} `json:"flows"`
	}
	if err := json.Unmarshal(respBody, &respData); err == nil {
		if respData.Session != "" {
			return respData.Session, fmt.Errorf(
				"UIA required: %s", respData.ErrCode,
			)
		}
	}

	return "", fmt.Errorf(
		"upload failed (%d): %s",
		resp.StatusCode,
		string(respBody),
	)
}
