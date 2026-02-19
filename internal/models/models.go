package models

import "time"

type UserStatus string

const (
	StatusOnline  UserStatus = "Online"
	StatusOffline UserStatus = "Offline"
	StatusAway    UserStatus = "Away"
)

type ChannelType string

const (
	ChannelText  ChannelType = "text"
	ChannelVoice ChannelType = "voice"
)

type User struct {
	ID         string
	Name       string
	Avatar     string
	Status     UserStatus
	Homeserver string
}

type Space struct {
	ID       string
	Name     string
	Avatar   string
	Status   string
	Address  string
	Nickname string
}

type Channel struct {
	ID      string
	Name    string
	Type    ChannelType
	SpaceID string
	Topic   string
}

type SpaceDetail struct {
	ID       string
	Name     string
	Avatar   string
	Address  string
	Channels []Channel
	Users    []User
}

type Reaction struct {
	Emoji          string
	Count          int
	HasCurrentUser bool
}

type Message struct {
	ID                 string
	Content            string
	Author             User
	Timestamp          time.Time
	ChannelID          string
	Reactions          []Reaction
	ThreadCount        int
	ThreadParticipants []User
	LastThreadReply    time.Time
	Attachments        []Attachment
	Embeds             []Embed
	IsPinned           bool
	IsSystem           bool
	SystemIcon         string
}

type AttachmentType string

const (
	AttachmentFile  AttachmentType = "file"
	AttachmentImage AttachmentType = "image"
)

type Attachment struct {
	Type     AttachmentType
	Name     string
	Size     string
	URL      string
	FileType string
	AltText  string
}

type Embed struct {
	Title       string
	Description string
	URL         string
	ImageURL    string
	SiteName    string
}

type LoginCredentials struct {
	Homeserver string
	Username   string
	Password   string
	DeviceID   string
}

type MatrixSession struct {
	Homeserver  string
	UserID      string
	AccessToken string
	DeviceID    string
}
