package models

import (
	"strings"
	"time"
	"unsafe"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

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
	E2EE       bool
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
	E2EE    bool
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
	Nonce              string
	RoomID             string
	Reactions          []Reaction
	ThreadCount        int
	ThreadParticipants []User
	LastThreadReply    time.Time
	Attachments        []Attachment
	Embeds             []Embed
	Undecryptable      bool
	Redacted           bool
	IsPinned           bool
	IsSystem           bool
	SystemIcon         string
}

func (m *Message) HTMLContent() string {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse([]byte(m.Content))

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	bs := markdown.Render(doc, renderer)

	return *(*string)(unsafe.Pointer(&bs))
}

func (m *Message) IsPending() bool {
	return strings.HasPrefix(m.ID, "pending-")
}

func (m *Message) IsDecrypting() bool {
	return strings.HasPrefix(m.ID, "decrypting-")
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
