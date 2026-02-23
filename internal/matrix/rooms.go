package matrix

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"slices"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/arko-chat/arko/internal/cache"
	"github.com/arko-chat/arko/internal/models"
)

func resolveContentURI(
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
	path := mxcToHTTP(uri)
	return "/api/media?path=" + url.QueryEscape(path)
}

func resolveContentURIString(
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
	path := mxcToHTTP(parsed)
	return "/api/media?path=" + url.QueryEscape(path)
}

func (m *Manager) GetCurrentUser(
	ctx context.Context,
	userID string,
) (models.User, error) {
	return cache.CachedSingle(m.userCache, m.userSfg, userID, func() (models.User, error) {
		client, err := m.GetClient(userID)
		if err != nil {
			return models.User{}, err
		}

		localpart := id.UserID(userID).Localpart()
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

		avatar := resolveContentURI(profile.AvatarURL, localpart, "avataaars")
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
	})
}

func (m *Manager) getRoomName(
	ctx context.Context,
	client *mautrix.Client,
	roomID id.RoomID,
) string {
	key := "name:" + roomID.String()
	val, _ := cache.CachedSingle(m.roomCache, m.roomNameSfg, key, func() (string, error) {
		var nameEvt event.RoomNameEventContent
		err := client.StateEvent(ctx, roomID, event.StateRoomName, "", &nameEvt)
		if err != nil || nameEvt.Name == "" {
			return roomID.String(), nil
		}
		return nameEvt.Name, nil
	})
	return val
}

func (m *Manager) getRoomAvatar(
	ctx context.Context,
	client *mautrix.Client,
	roomID id.RoomID,
) string {
	key := "avatar:" + roomID.String()
	val, _ := cache.CachedSingle(m.roomCache, m.roomAvatarSfg, key, func() (string, error) {
		var avatarEvt event.RoomAvatarEventContent
		err := client.StateEvent(ctx, roomID, event.StateRoomAvatar, "", &avatarEvt)
		if err != nil {
			return fmt.Sprintf(
				"https://api.dicebear.com/7.x/shapes/svg?seed=%s",
				roomID.String(),
			), nil
		}
		return resolveContentURIString(avatarEvt.URL, roomID.String(), "shapes"), nil
	})
	return val
}

func (m *Manager) ListSpaces(
	ctx context.Context,
	userID string,
) ([]models.Space, error) {
	return cache.CachedSingle(m.spacesCache, m.spacesSfg, userID, func() ([]models.Space, error) {
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
			err := client.StateEvent(ctx, roomID, event.StateCreate, "", &createEvt)
			if err != nil || createEvt.Type != event.RoomTypeSpace {
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

		slices.SortFunc(spaces, func(a, b models.Space) int {
			return cmp.Compare(a.Name, b.Name)
		})

		return spaces, nil
	})
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

	roomID := id.RoomID(spaceID)
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
		ID:       spaceID,
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
	return cache.CachedSingle(m.channelsCache, m.channelsSfg, spaceID.String(), func() ([]models.Channel, error) {
		stateMap, err := client.State(ctx, spaceID)
		if err != nil {
			return nil, err
		}

		childEvents, ok := stateMap[event.StateSpaceChild]
		if !ok {
			return nil, nil
		}

		var channels []models.Channel
		for stateKey, childEvent := range childEvents {
			content, ok := childEvent.Content.Parsed.(*event.SpaceChildEventContent)
			if !ok || content == nil || len(content.Via) == 0 {
				continue
			}

			childRoomID := id.RoomID(stateKey)
			childName := m.getRoomName(ctx, client, childRoomID)

			channels = append(channels, models.Channel{
				ID:      childRoomID.String(),
				Name:    childName,
				Type:    models.ChannelText,
				SpaceID: spaceID.String(),
			})
		}

		slices.SortFunc(channels, func(a, b models.Channel) int {
			return cmp.Compare(a.Name, b.Name)
		})

		return channels, nil
	})
}

func (m *Manager) ListDirectMessages(
	ctx context.Context,
	userID string,
) ([]models.User, error) {
	return cache.CachedSingle(m.dmCache, m.dmSfg, userID, func() ([]models.User, error) {
		client, err := m.GetClient(userID)
		if err != nil {
			return nil, err
		}

		var dmMap map[id.UserID][]id.RoomID
		err = client.GetAccountData(ctx, event.AccountDataDirectChats.Type, &dmMap)
		if err != nil {
			return nil, fmt.Errorf("get dm list: %w", err)
		}

		var friends []models.User
		seen := make(map[string]bool)
		for otherUser := range dmMap {
			uid := otherUser.String()
			if seen[uid] {
				continue
			}
			seen[uid] = true

			// Use the session's profile cache to resolve the friend's details
			sess, ok := m.matrixSessions.Load(userID)
			if ok {
				profile, err := sess.GetUserProfile(ctx, uid)
				if err == nil {
					friends = append(friends, profile)
					continue
				}
			}

			// Fallback if session/profile fetch fails
			localpart := otherUser.Localpart()
			friends = append(friends, models.User{
				ID:     uid,
				Name:   localpart,
				Avatar: fmt.Sprintf("https://api.dicebear.com/7.x/avataaars/svg?seed=%s", localpart),
				Status: models.StatusOffline,
			})
		}

		slices.SortFunc(friends, func(a, b models.User) int {
			return cmp.Compare(a.Name, b.Name)
		})

		return friends, nil
	})
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

	roomID := id.RoomID(channelID)
	name := m.getRoomName(ctx, client, roomID)

	var topicEvt event.TopicEventContent
	_ = client.StateEvent(ctx, roomID, event.StateTopic, "", &topicEvt)

	return models.Channel{
		ID:      channelID,
		Name:    name,
		Type:    models.ChannelText,
		SpaceID: spaceID,
		Topic:   topicEvt.Topic,
	}, nil
}

func (m *Manager) getRoomMembers(
	ctx context.Context,
	client *mautrix.Client,
	roomID id.RoomID,
) ([]models.User, error) {
	return cache.CachedSingle(m.membersCache, m.membersSfg, roomID.String(), func() ([]models.User, error) {
		members, err := client.Members(ctx, roomID)
		if err != nil {
			return nil, err
		}

		var users []models.User
		for _, evt := range members.Chunk {
			content, ok := evt.Content.Parsed.(*event.MemberEventContent)
			if !ok || content.Membership != event.MembershipJoin {
				continue
			}

			stateKey := evt.GetStateKey()
			localpart := id.UserID(stateKey).Localpart()

			name := stateKey
			if content.Displayname != "" {
				name = content.Displayname
			}

			avatar := resolveContentURIString(content.AvatarURL, localpart, "avataaars")

			users = append(users, models.User{
				ID:     stateKey,
				Name:   name,
				Avatar: avatar,
				Status: models.StatusOnline,
			})
		}

		return users, nil
	})
}
