package matrix

import (
	"fmt"

	"github.com/arko-chat/arko/internal/models"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

func (m *MatrixSession) SearchUsers(query string) ([]models.User, error) {
	ctx := m.context

	resp, err := m.client.SearchUserDirectory(ctx, query, 10)
	if err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}

	var users []models.User
	for _, entry := range resp.Results {
		if entry.UserID.String() == m.id {
			continue
		}

		name := entry.UserID.Localpart()
		if entry.DisplayName != "" {
			name = entry.DisplayName
		}

		avatar := resolveContentURI(entry.AvatarURL, entry.UserID.Localpart(), "avataaars")

		users = append(users, models.User{
			ID:     entry.UserID.String(),
			Name:   name,
			Avatar: avatar,
			Status: models.StatusOffline,
		})
	}

	return users, nil
}

func (m *MatrixSession) CreateDMRoom(otherUserID string) (models.User, string, error) {
	ctx := m.context
	target := id.UserID(otherUserID)

	profile, err := m.client.GetProfile(ctx, target)
	if err != nil {
		profile = &mautrix.RespUserProfile{}
	}

	name := target.Localpart()
	if profile.DisplayName != "" {
		name = profile.DisplayName
	}
	avatar := resolveContentURI(profile.AvatarURL, target.Localpart(), "avataaars")

	req := &mautrix.ReqCreateRoom{
		Preset:   "trusted_private_chat",
		IsDirect: true,
		Invite:   []id.UserID{target},
		InitialState: []*event.Event{
			{
				Type: event.StateEncryption,
				Content: event.Content{
					Parsed: &event.EncryptionEventContent{
						Algorithm: id.AlgorithmMegolmV1,
					},
				},
			},
		},
	}

	resp, err := m.client.CreateRoom(ctx, req)
	if err != nil {
		return models.User{}, "", fmt.Errorf("create dm room: %w", err)
	}

	if err := m.setDMRoomAccountData(target, resp.RoomID); err != nil {
		m.logger.Warn("failed to update m.direct account data", "error", err)
	}

	m.dmCache.Invalidate("ldm:" + m.id)

	user := models.User{
		ID:     otherUserID,
		Name:   name,
		Avatar: avatar,
		Status: models.StatusOffline,
	}

	return user, resp.RoomID.String(), nil
}

func (m *MatrixSession) setDMRoomAccountData(userID id.UserID, roomID id.RoomID) error {
	ctx := m.context

	var dmMap map[id.UserID][]id.RoomID
	err := m.client.GetAccountData(ctx, event.AccountDataDirectChats.Type, &dmMap)
	if err != nil {
		dmMap = make(map[id.UserID][]id.RoomID)
	}

	rooms := dmMap[userID]
	for _, r := range rooms {
		if r == roomID {
			return nil
		}
	}

	dmMap[userID] = append([]id.RoomID{roomID}, rooms...)

	return m.client.SetAccountData(ctx, event.AccountDataDirectChats.Type, dmMap)
}
