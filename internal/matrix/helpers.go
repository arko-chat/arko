package matrix

import (
	"fmt"
	"net/url"

	"maunium.net/go/mautrix/id"
)

func mxcToHTTP(uri id.ContentURI) string {
	if uri.IsEmpty() {
		return ""
	}
	return fmt.Sprintf(
		"/_matrix/client/v1/media/download/%s/%s",
		uri.Homeserver,
		uri.FileID,
	)
}

func encodeRoomID(roomID string) string {
	return url.PathEscape(roomID)
}

func decodeRoomID(encoded string) string {
	decoded, err := url.PathUnescape(encoded)
	if err != nil {
		return encoded
	}
	return decoded
}
