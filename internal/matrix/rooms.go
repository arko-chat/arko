package matrix

import (
	"cmp"
	"fmt"
	"math/rand/v2"
	"slices"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/arko-chat/arko/internal/models"
)

func (m *MatrixSession) GetCurrentUser() (models.User, error) {
	return m.userCache.Get("gcu:"+m.id, func() (models.User, error) {
		ctx := m.context
		localpart := id.UserID(m.id).Localpart()

		profile, err := m.client.GetProfile(ctx, id.UserID(m.id))
		if err != nil {
			return models.User{
				ID:   m.id,
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

		user := models.User{
			ID:     m.id,
			Name:   name,
			Avatar: avatar,
			Status: models.StatusOnline,
		}

		presence, err := m.client.GetPresence(ctx, id.UserID(m.id))
		if err == nil {
			if presence.CurrentlyActive {
				user.Status = models.StatusOnline
			} else {
				user.Status = models.StatusOffline
			}
		}

		return user, nil
	})
}

func (m *MatrixSession) getRoomName(roomID id.RoomID) string {
	key := "grn:" + roomID.String()
	val, _ := m.roomCache.Get(key, func() (string, error) {
		var nameEvt event.RoomNameEventContent
		err := m.client.StateEvent(
			m.context, roomID, event.StateRoomName, "", &nameEvt,
		)
		if err != nil || nameEvt.Name == "" {
			return roomID.String(), nil
		}
		return nameEvt.Name, nil
	})
	return val
}

func (m *MatrixSession) getRoomAvatar(roomID id.RoomID) string {
	key := "gra:" + roomID.String()
	val, _ := m.roomCache.Get(key, func() (string, error) {
		var avatarEvt event.RoomAvatarEventContent
		err := m.client.StateEvent(
			m.context, roomID, event.StateRoomAvatar, "", &avatarEvt,
		)
		if err != nil {
			return fmt.Sprintf(
				"https://api.dicebear.com/7.x/shapes/svg?seed=%s",
				roomID.String(),
			), nil
		}
		return resolveContentURIString(
			avatarEvt.URL, roomID.String(), "shapes",
		), nil
	})
	return val
}

func (m *MatrixSession) ListSpaces() ([]models.Space, error) {
	return m.spacesCache.Get("ls:"+m.id, func() ([]models.Space, error) {
		resp, err := m.client.JoinedRooms(m.context)
		if err != nil {
			return nil, fmt.Errorf("joined rooms: %w", err)
		}

		var spaces []models.Space
		for _, roomID := range resp.JoinedRooms {
			var createEvt event.CreateEventContent
			err := m.client.StateEvent(
				m.context, roomID, event.StateCreate, "", &createEvt,
			)
			if err != nil || createEvt.Type != event.RoomTypeSpace {
				continue
			}

			name := m.getRoomName(roomID)
			avatar := m.getRoomAvatar(roomID)

			spaces = append(spaces, models.Space{
				ID:      roomID.String(),
				Name:    name,
				Avatar:  avatar,
				Status:  "Online",
				Address: encodeRoomID(roomID.String()),
			})

			go m.getSpaceChildren(roomID)
			go m.getRoomMembers(roomID)
		}

		slices.SortFunc(spaces, func(a, b models.Space) int {
			return cmp.Compare(a.Name, b.Name)
		})

		return spaces, nil
	})
}

func (m *MatrixSession) GetSpaceDetail(spaceID string) (models.SpaceDetail, error) {
	roomID := id.RoomID(spaceID)
	name := m.getRoomName(roomID)
	avatar := m.getRoomAvatar(roomID)

	children, err := m.getSpaceChildren(roomID)
	if err != nil {
		children = nil
	}

	members, err := m.getRoomMembers(roomID)
	if err != nil {
		members = nil
	}

	shareUrl := ""
	urls, err := m.getUrls(roomID)
	if err == nil && len(urls) > 0 {
		shareUrl = urls[0]
	}

	return models.SpaceDetail{
		ID:        spaceID,
		Name:      name,
		Avatar:    avatar,
		Channels:  children,
		Users:     members,
		InviteURL: shareUrl,
	}, nil
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.IntN(len(letters))]
	}
	return string(b)
}

func (m *MatrixSession) getUrls(roomID id.RoomID) ([]string, error) {
	return m.aliasesCache.Get("gu:"+roomID.String(), func() ([]string, error) {
		m.logger.Debug("fetching aliases", "roomID", roomID)

		resp, err := m.client.GetAliases(m.context, roomID)
		if err != nil {
			return nil, err
		}

		if len(resp.Aliases) == 0 {
			m.logger.Debug("no aliases found, creating one", "roomID", roomID)

			home := m.client.UserID.Homeserver()
			m.logger.Debug("creating alias", "roomID", roomID, "host", home)
			alias := id.NewRoomAlias(randString(8), home)

			if _, err := m.client.CreateAlias(m.context, alias, roomID); err != nil {
				m.logger.Debug("failed to create alias", "roomID", roomID, "error", err)
				return nil, fmt.Errorf("failed to create alias: %w", err)
			}

			m.logger.Debug("alias created", "roomID", roomID, "alias", alias)
			return []string{alias.URI().MatrixToURL()}, nil
		}

		m.logger.Debug("aliases found", "roomID", roomID, "count", len(resp.Aliases))

		urls := make([]string, 0, len(resp.Aliases))
		for _, r := range resp.Aliases {
			urls = append(urls, r.URI().MatrixToURL())
		}

		return urls, nil
	})
}

func (m *MatrixSession) getSpaceChildren(
	spaceID id.RoomID,
) ([]models.Channel, error) {
	return m.channelsCache.Get("gsc:"+spaceID.String(), func() ([]models.Channel, error) {
		stateMap, err := m.client.State(m.context, spaceID)
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
			childName := m.getRoomName(childRoomID)

			channels = append(channels, models.Channel{
				ID:      childRoomID.String(),
				Name:    childName,
				Type:    models.ChannelText,
				SpaceID: spaceID.String(),
			})

			go func() {
				tree := m.GetMessageTree(string(childRoomID))
				tree.Initialize(m.context)
			}()
		}

		slices.SortFunc(channels, func(a, b models.Channel) int {
			return cmp.Compare(a.Name, b.Name)
		})

		return channels, nil
	})
}

func (m *MatrixSession) ListDirectMessages() ([]models.User, error) {
	return m.dmCache.Get("ldm:"+m.id, func() ([]models.User, error) {
		var dmMap map[id.UserID][]id.RoomID
		err := m.client.GetAccountData(
			m.context, event.AccountDataDirectChats.Type, &dmMap,
		)
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

			profile, err := m.GetUserProfile(uid)
			if err != nil {
				continue
			}
			friends = append(friends, profile)

			go func() {
				roomID, err := m.GetDMRoomID(uid)
				if err == nil {
					tree := m.GetMessageTree(string(roomID))
					tree.Initialize(m.context)
				}
			}()
		}

		slices.SortFunc(friends, func(a, b models.User) int {
			return cmp.Compare(a.Name, b.Name)
		})

		return friends, nil
	})
}

func (m *MatrixSession) GetDMRoomID(otherUserID string) (string, error) {
	var dmMap map[id.UserID][]id.RoomID
	err := m.client.GetAccountData(
		m.context, event.AccountDataDirectChats.Type, &dmMap,
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

func (m *MatrixSession) GetChannel(
	spaceID string,
	channelID string,
) (models.Channel, error) {
	roomID := id.RoomID(channelID)
	name := m.getRoomName(roomID)

	var topicEvt event.TopicEventContent
	_ = m.client.StateEvent(
		m.context, roomID, event.StateTopic, "", &topicEvt,
	)

	return models.Channel{
		ID:      channelID,
		Name:    name,
		Type:    models.ChannelText,
		SpaceID: spaceID,
		Topic:   topicEvt.Topic,
	}, nil
}

func (m *MatrixSession) getRoomMembers(
	roomID id.RoomID,
) ([]models.User, error) {
	return m.membersCache.Get("grm:"+roomID.String(), func() ([]models.User, error) {
		members, err := m.client.Members(m.context, roomID)
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

			avatar := resolveContentURIString(
				content.AvatarURL, localpart, "avataaars",
			)

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
