package ws

import "encoding/json"

func RedirectMessage(path string) []byte {
	b, _ := json.Marshal(map[string]string{"redirect": path})
	return b
}
