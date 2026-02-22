package service

import (
	"crypto/sha256"
	"encoding/hex"
)

func SafeHashClass(input string) string {
	hash := sha256.Sum256([]byte(input))
	hashString := hex.EncodeToString(hash[:])
	return hashString
}
