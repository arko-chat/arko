package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"maunium.net/go/mautrix"

	"github.com/arko-chat/arko/internal/session"
)

type refreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresInMs  int64  `json:"expires_in_ms,omitempty"`
}

func (m *Manager) startTokenRefresh(
	ctx context.Context,
	sess *session.Session,
	client *mautrix.Client,
) {
	if sess.RefreshToken == "" || sess.ExpiresInMs <= 0 {
		return
	}

	go m.tokenRefreshLoop(ctx, sess.UserID, client, sess.RefreshToken, sess.ExpiresInMs)
}

func (m *Manager) tokenRefreshLoop(
	ctx context.Context,
	userID string,
	client *mautrix.Client,
	refreshToken string,
	expiresInMs int64,
) {
	const minWait = 30 * time.Second
	const retryDelay = 30 * time.Second
	const refreshFraction = 80

	wait := refreshWait(expiresInMs, refreshFraction, minWait)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}

		resp, err := m.doRefreshToken(ctx, client, refreshToken)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			m.logger.Error("token refresh failed",
				"user", userID,
				"err", err,
			)
			wait = retryDelay
			continue
		}

		client.AccessToken = resp.AccessToken

		if resp.RefreshToken != "" {
			refreshToken = resp.RefreshToken
		}

		if err := session.Update(userID, func(s *session.Session) {
			s.AccessToken = resp.AccessToken
			if resp.RefreshToken != "" {
				s.RefreshToken = resp.RefreshToken
			}
			if resp.ExpiresInMs > 0 {
				s.ExpiresInMs = resp.ExpiresInMs
			}
		}); err != nil {
			m.logger.Warn("failed to persist refreshed tokens",
				"user", userID,
				"err", err,
			)
		}

		m.logger.Info("token refreshed successfully", "user", userID)

		if resp.ExpiresInMs > 0 {
			expiresInMs = resp.ExpiresInMs
		}
		wait = refreshWait(expiresInMs, refreshFraction, minWait)
	}
}

func (m *Manager) doRefreshToken(
	ctx context.Context,
	client *mautrix.Client,
	refreshToken string,
) (*refreshResponse, error) {
	hsURL := strings.TrimRight(client.HomeserverURL.String(), "/")
	endpoint := hsURL + "/_matrix/client/v3/refresh"

	reqBody, err := json.Marshal(map[string]string{
		"refresh_token": refreshToken,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"refresh failed (%d): %s", resp.StatusCode, body,
		)
	}

	var result refreshResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if result.AccessToken == "" {
		return nil, fmt.Errorf("empty access token in response")
	}

	return &result, nil
}

func refreshWait(
	expiresInMs int64,
	pct int,
	floor time.Duration,
) time.Duration {
	d := time.Duration(expiresInMs) * time.Millisecond * time.Duration(pct) / 100
	if d < floor {
		return floor
	}
	return d
}
