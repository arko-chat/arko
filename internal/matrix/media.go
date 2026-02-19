package matrix

import (
	"fmt"
	"net/url"
	"strings"

	"maunium.net/go/mautrix/id"
)

func mxcToHTTP(homeserverURL string, uri id.ContentURI) string {
	if uri.IsEmpty() {
		return ""
	}
	hsURL := strings.TrimRight(homeserverURL, "/")
	return fmt.Sprintf(
		"%s/_matrix/media/v3/download/%s/%s",
		hsURL,
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
