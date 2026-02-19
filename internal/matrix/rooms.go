package matrix

import (
	"context"
	"fmt"
	"sort"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/arko-chat/arko/internal/models"
)

func resolveContentURI(
	hsURL string,
	uri id.ContentURI,
	fallbackSeed string,
	fallbackStyle string,
) string {
	if uri.IsEmpty() {
		return fmt.Sprintf(
			"https://api.dicebear.com/7.x/%s/svg?seed=%s",
			fallbackStyle, fallbackSeed,
		)
	}
	return mxcToHTTP(hsURL, uri)
}

func resolveContentURIString(
	hsURL string,
	uriStr id.ContentURIString,
	fallbackSeed string,
	fallbackStyle string,
) string {
	if uriStr == "" {
		return fmt.Sprintf(
			"https://api.dicebear.com/7.x/%s/svg?seed=%s",
			fallbackStyle, fallbackSeed,
		)
	}
	parsed, err := id.ParseContentURI(string(uriStr))
	if err != nil {
		return fmt.Sprintf(
			"https://api.dicebear.com/7.x/%s/svg?seed=%s",
			fallbackStyle, fallbackSeed,
		)
	}
	return mxcToHTTP(hsURL, parsed)
}

func (m *Manager) GetCurrentUser(
	ctx context.Context,
	userID string,
) (models.User, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return models.User{}, err
	}

	localpart := id.UserID(userID).Localpart()
	hsURL := client.HomeserverURL.String()

	profile, err := client.GetProfile(ctx, id.UserID(userID))
	if err != nil {
		return models.User{
			ID:   userID,
			Name: localpart,
			Avatar: fmt.Sprintf(
				"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
				localpart,
			),
			Status: models.StatusOnline,
		}, nil
	}

	avatar := resolveContentURI(
		hsURL, profile.AvatarURL, localpart, "avataaars",
	)

	name := profile.DisplayName
	if name == "" {
		name = localpart
	}

	return models.User{
		ID:     userID,
		Name:   name,
		Avatar: avatar,
		Status: models.StatusOnline,
	}, nil
}

func (m *Manager) ListSpaces(
	ctx context.Context,
	userID string,
) ([]models.Space, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return nil, err
	}

	resp, err := client.JoinedRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("joined rooms: %w", err)
	}

	var spaces []models.Space
	for _, roomID := range resp.JoinedRooms {
		var createEvt event.CreateEventContent
		err := client.StateEvent(
			ctx,
			roomID,
			event.StateCreate,
			"",
			&createEvt,
		)
		if err != nil {
			continue
		}

		if createEvt.Type != event.RoomTypeSpace {
			continue
		}

		name := m.getRoomName(ctx, client, roomID)
		avatar := m.getRoomAvatar(ctx, client, roomID)

		spaces = append(spaces, models.Space{
			ID:      roomID.String(),
			Name:    name,
			Avatar:  avatar,
			Status:  "Online",
			Address: encodeRoomID(roomID.String()),
		})
	}

	return spaces, nil
}

func (m *Manager) GetSpaceDetail(
	ctx context.Context,
	userID string,
	spaceID string,
) (models.SpaceDetail, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return models.SpaceDetail{}, err
	}

	actualID := decodeRoomID(spaceID)
	roomID := id.RoomID(actualID)
	name := m.getRoomName(ctx, client, roomID)
	avatar := m.getRoomAvatar(ctx, client, roomID)

	children, err := m.getSpaceChildren(ctx, client, roomID)
	if err != nil {
		children = nil
	}

	members, err := m.getRoomMembers(ctx, client, roomID)
	if err != nil {
		members = nil
	}

	return models.SpaceDetail{
		ID:       actualID,
		Name:     name,
		Avatar:   avatar,
		Channels: children,
		Users:    members,
	}, nil
}

func (m *Manager) getSpaceChildren(
	ctx context.Context,
	client *mautrix.Client,
	spaceID id.RoomID,
) ([]models.Channel, error) {
	stateMap, err := client.State(ctx, spaceID)
	if err != nil {
		return nil, err
	}

	childEvents, ok := stateMap[event.StateSpaceChild]
	if !ok {
		return nil, nil
	}

	var channels []models.Channel
	for stateKey := range childEvents {
		childRoomID := id.RoomID(stateKey)
		childName := m.getRoomName(ctx, client, childRoomID)

		channels = append(channels, models.Channel{
			ID:      childRoomID.String(),
			Name:    childName,
			Type:    models.ChannelText,
			SpaceID: spaceID.String(),
		})
	}

	return channels, nil
}

func (m *Manager) ListDirectMessages(
	ctx context.Context,
	userID string,
) ([]models.User, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return nil, err
	}

	var dmMap map[id.UserID][]id.RoomID
	err = client.GetAccountData(
		ctx,
		event.AccountDataDirectChats.Type,
		&dmMap,
	)
	if err != nil {
		return nil, fmt.Errorf("get dm list: %w", err)
	}

	hsURL := client.HomeserverURL.String()

	var friends []models.User
	seen := make(map[string]bool)
	for otherUser := range dmMap {
		uid := otherUser.String()
		if seen[uid] {
			continue
		}
		seen[uid] = true

		localpart := otherUser.Localpart()
		name := localpart
		avatar := fmt.Sprintf(
			"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
			localpart,
		)

		profile, _ := client.GetProfile(ctx, otherUser)
		if profile != nil {
			if profile.DisplayName != "" {
				name = profile.DisplayName
			}
			avatar = resolveContentURI(
				hsURL, profile.AvatarURL, localpart, "avataaars",
			)
		}

		friends = append(friends, models.User{
			ID:     uid,
			Name:   name,
			Avatar: avatar,
			Status: models.StatusOffline,
		})
	}

	return friends, nil
}

func (m *Manager) GetDMRoomID(
	ctx context.Context,
	userID string,
	otherUserID string,
) (string, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return "", err
	}

	var dmMap map[id.UserID][]id.RoomID
	err = client.GetAccountData(
		ctx,
		event.AccountDataDirectChats.Type,
		&dmMap,
	)
	if err != nil {
		return "", err
	}

	rooms, ok := dmMap[id.UserID(otherUserID)]
	if !ok || len(rooms) == 0 {
		return "", fmt.Errorf("no DM room with %s", otherUserID)
	}

	return rooms[0].String(), nil
}

func (m *Manager) GetChannel(
	ctx context.Context,
	userID string,
	spaceID string,
	channelID string,
) (models.Channel, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return models.Channel{}, err
	}

	actualChannelID := decodeRoomID(channelID)
	actualSpaceID := decodeRoomID(spaceID)
	roomID := id.RoomID(actualChannelID)
	name := m.getRoomName(ctx, client, roomID)

	var topicEvt event.TopicEventContent
	_ = client.StateEvent(ctx, roomID, event.StateTopic, "", &topicEvt)

	return models.Channel{
		ID:      actualChannelID,
		Name:    name,
		Type:    models.ChannelText,
		SpaceID: actualSpaceID,
		Topic:   topicEvt.Topic,
	}, nil
}

func (m *Manager) GetRoomMessages(
	ctx context.Context,
	userID string,
	roomID string,
	limit int,
) ([]models.Message, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return nil, err
	}

	actualRoomID := decodeRoomID(roomID)
	hsURL := client.HomeserverURL.String()

	m.mu.RLock()
	helper := m.cryptoHelpers[userID]
	m.mu.RUnlock()

	resp, err := client.Messages(
		ctx,
		id.RoomID(actualRoomID),
		"",
		"",
		'b',
		nil,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}

	var messages []models.Message
	for _, evt := range resp.Chunk {
		if evt.Type == event.EventEncrypted && helper != nil {
			_ = evt.Content.ParseRaw(evt.Type)
			decrypted, decErr := helper.Decrypt(ctx, evt)
			if decErr != nil {
				encContent, ok := evt.Content.Parsed.(*event.EncryptedEventContent)
				if ok {
					helper.RequestSession(
						ctx,
						id.RoomID(actualRoomID),
						encContent.SenderKey,
						encContent.SessionID,
						evt.Sender,
						encContent.DeviceID,
					)
				}

				messages = append(messages, models.Message{
					ID:      evt.ID.String(),
					Content: "🔒 Unable to decrypt this message.",
					Author: models.User{
						ID:   evt.Sender.String(),
						Name: evt.Sender.Localpart(),
						Avatar: fmt.Sprintf(
							"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
							evt.Sender.Localpart(),
						),
						Status: models.StatusOnline,
					},
					Timestamp: time.UnixMilli(evt.Timestamp),
					ChannelID: actualRoomID,
				})
				continue
			}
			evt = decrypted
		}

		if evt.Type != event.EventMessage {
			continue
		}

		_ = evt.Content.ParseRaw(evt.Type)

		content, ok := evt.Content.Parsed.(*event.MessageEventContent)
		if !ok {
			continue
		}

		senderName := evt.Sender.Localpart()
		avatarURL := fmt.Sprintf(
			"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
			senderName,
		)

		profile, _ := client.GetProfile(ctx, evt.Sender)
		if profile != nil {
			if profile.DisplayName != "" {
				senderName = profile.DisplayName
			}
			avatarURL = resolveContentURI(
				hsURL, profile.AvatarURL, evt.Sender.Localpart(), "avataaars",
			)
		}

		messages = append(messages, models.Message{
			ID:      evt.ID.String(),
			Content: content.Body,
			Author: models.User{
				ID:     evt.Sender.String(),
				Name:   senderName,
				Avatar: avatarURL,
				Status: models.StatusOnline,
			},
			Timestamp: time.UnixMilli(evt.Timestamp),
			ChannelID: actualRoomID,
		})
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Timestamp.Before(messages[j].Timestamp)
	})

	return messages, nil
}

func (m *Manager) SendMessage(
	ctx context.Context,
	userID string,
	roomID string,
	body string,
) error {
	client, err := m.GetClient(userID)
	if err != nil {
		return err
	}

	actualRoomID := decodeRoomID(roomID)
	rid := id.RoomID(actualRoomID)

	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    body,
	}

	m.mu.RLock()
	helper := m.cryptoHelpers[userID]
	m.mu.RUnlock()

	if helper != nil {
		var encEvt event.EncryptionEventContent
		err := client.StateEvent(
			ctx, rid, event.StateEncryption, "", &encEvt,
		)
		if err == nil && encEvt.Algorithm != "" {
			encrypted, encErr := helper.Encrypt(
				ctx, rid, event.EventMessage, content,
			)
			if encErr != nil {
				return fmt.Errorf("encrypt: %w", encErr)
			}
			_, err = client.SendMessageEvent(
				ctx, rid, event.EventEncrypted, encrypted,
			)
			return err
		}
	}

	_, err = client.SendMessageEvent(
		ctx, rid, event.EventMessage, content,
	)
	return err
}

func (m *Manager) GetUserProfile(
	ctx context.Context,
	userID string,
	targetUserID string,
) (models.User, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return models.User{}, err
	}

	target := id.UserID(targetUserID)
	localpart := target.Localpart()
	hsURL := client.HomeserverURL.String()

	name := localpart
	avatar := fmt.Sprintf(
		"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
		localpart,
	)

	profile, err := client.GetProfile(ctx, target)
	if err == nil && profile != nil {
		if profile.DisplayName != "" {
			name = profile.DisplayName
		}
		avatar = resolveContentURI(
			hsURL, profile.AvatarURL, localpart, "avataaars",
		)
	}

	return models.User{
		ID:     targetUserID,
		Name:   name,
		Avatar: avatar,
		Status: models.StatusOffline,
	}, nil
}

func (m *Manager) ListJoinedRooms(
	ctx context.Context,
	userID string,
) ([]string, error) {
	client, err := m.GetClient(userID)
	if err != nil {
		return nil, err
	}

	resp, err := client.JoinedRooms(ctx)
	if err != nil {
		return nil, err
	}

	var rooms []string
	for _, r := range resp.JoinedRooms {
		rooms = append(rooms, r.String())
	}
	return rooms, nil
}

func (m *Manager) getRoomName(
	ctx context.Context,
	client *mautrix.Client,
	roomID id.RoomID,
) string {
	var nameEvt event.RoomNameEventContent
	err := client.StateEvent(
		ctx,
		roomID,
		event.StateRoomName,
		"",
		&nameEvt,
	)
	if err != nil || nameEvt.Name == "" {
		return roomID.String()
	}
	return nameEvt.Name
}

func (m *Manager) getRoomAvatar(
	ctx context.Context,
	client *mautrix.Client,
	roomID id.RoomID,
) string {
	var avatarEvt event.RoomAvatarEventContent
	err := client.StateEvent(
		ctx,
		roomID,
		event.StateRoomAvatar,
		"",
		&avatarEvt,
	)
	if err != nil {
		return fmt.Sprintf(
			"https://api.dicebear.com/7.x/shapes/svg?seed=%s",
			roomID.String(),
		)
	}

	return resolveContentURIString(
		client.HomeserverURL.String(),
		avatarEvt.URL,
		roomID.String(),
		"shapes",
	)
}

func (m *Manager) getRoomMembers(
	ctx context.Context,
	client *mautrix.Client,
	roomID id.RoomID,
) ([]models.User, error) {
	members, err := client.Members(ctx, roomID)
	if err != nil {
		return nil, err
	}

	hsURL := client.HomeserverURL.String()

	var users []models.User
	for _, evt := range members.Chunk {
		content, ok := evt.Content.Parsed.(*event.MemberEventContent)
		if !ok || content.Membership != event.MembershipJoin {
			continue
		}

		stateKey := evt.GetStateKey()
		localpart := id.UserID(stateKey).Localpart()
		name := stateKey
		avatar := fmt.Sprintf(
			"https://api.dicebear.com/7.x/avataaars/svg?seed=%s",
			localpart,
		)

		if content.Displayname != "" {
			name = content.Displayname
		}

		avatar = resolveContentURIString(
			hsURL, content.AvatarURL, localpart, "avataaars",
		)

		users = append(users, models.User{
			ID:     stateKey,
			Name:   name,
			Avatar: avatar,
			Status: models.StatusOnline,
		})
	}

	return users, nil
}
