package matrix

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"net/url"

	"maunium.net/go/mautrix/id"
)

func safeHashClass(input string) string {
	h := fnv.New32a()
	h.Write([]byte(input))
	return fmt.Sprintf("c%x", h.Sum32())
}

func generateNonce() (string, error) {
	nonceBytes := make([]byte, 16)

	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("could not generate random nonce: %w", err)
	}

	return "pending-" + safeHashClass(base64.RawURLEncoding.EncodeToString(nonceBytes)), nil
}

func generateDecryptingNonce() (string, error) {
	nonceBytes := make([]byte, 16)

	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("could not generate random nonce: %w", err)
	}

	return "decrypting-" + safeHashClass(base64.RawURLEncoding.EncodeToString(nonceBytes)), nil
}

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
