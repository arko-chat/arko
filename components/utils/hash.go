package utils

import (
	"fmt"
	"hash/fnv"
)

func Hash(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum64())
}
