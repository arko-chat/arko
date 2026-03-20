package matrix

import (
	"context"
	"fmt"

	"github.com/arko-chat/arko/internal/models"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type CreateSpaceParams struct {
	Name      string
	Topic     string
	AvatarURL string
	Public    bool
}

type CreateChannelParams struct {
	Name    string
	Topic   string
	SpaceID string
	Public  bool
}

func (m *MatrixSession) CreateSpace(params CreateSpaceParams) (models.Space, error) {
	ctx := m.context

	preset := "private_chat"
	if params.Public {
		preset = "public_chat"
	}

	creationContent := map[string]interface{}{
		"type": event.RoomTypeSpace,
	}

	req := &mautrix.ReqCreateRoom{
		Name:            params.Name,
		Topic:           params.Topic,
		Preset:          preset,
		CreationContent: creationContent,
	}

	resp, err := m.client.CreateRoom(ctx, req)
	if err != nil {
		return models.Space{}, fmt.Errorf("create space: %w", err)
	}

	m.spacesCache.Invalidate("ls:" + m.id)

	return models.Space{
		ID:      resp.RoomID.String(),
		Name:    params.Name,
		Avatar:  params.AvatarURL,
		Status:  "Online",
		Address: encodeRoomID(resp.RoomID.String()),
	}, nil
}

func (m *MatrixSession) CreateChannel(params CreateChannelParams) (models.Channel, error) {
	ctx := m.context

	preset := "private_chat"
	if params.Public {
		preset = "public_chat"
	}

	initialState := []*event.Event{
		{
			Type:     event.StateSpaceParent,
			StateKey: ptr(params.SpaceID),
			Content: event.Content{
				Parsed: &event.SpaceParentEventContent{
					Via: []string{m.client.UserID.Homeserver()},
				},
			},
		},
	}

	req := &mautrix.ReqCreateRoom{
		Name:         params.Name,
		Topic:        params.Topic,
		Preset:       preset,
		InitialState: initialState,
	}

	resp, err := m.client.CreateRoom(ctx, req)
	if err != nil {
		return models.Channel{}, fmt.Errorf("create channel: %w", err)
	}

	if err := m.addChildToSpace(ctx, id.RoomID(params.SpaceID), resp.RoomID); err != nil {
		m.logger.Warn("failed to add space child relation", "error", err)
	}

	m.channelsCache.Invalidate("gsc:" + params.SpaceID)

	return models.Channel{
		ID:      resp.RoomID.String(),
		Name:    params.Name,
		Type:    models.ChannelText,
		SpaceID: params.SpaceID,
		Topic:   params.Topic,
	}, nil
}

func (m *MatrixSession) addChildToSpace(ctx context.Context, spaceID, childID id.RoomID) error {
	_, err := m.client.SendStateEvent(
		ctx,
		spaceID,
		event.StateSpaceChild,
		childID.String(),
		&event.SpaceChildEventContent{
			Via: []string{m.client.UserID.Homeserver()},
		},
	)
	return err
}

func ptr[T any](v T) *T {
	return &v
}
