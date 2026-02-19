package credentials

import (
	"encoding/json"
	"slices"

	"github.com/zalando/go-keyring"
)

const knownUsersKey = "app:known_users"

func AddKnownUser(userID string) error {
	users := GetKnownUsers()
	if slices.Contains(users, userID) {
		return nil
	}
	users = append(users, userID)
	data, _ := json.Marshal(users)
	return keyring.Set(serviceName, knownUsersKey, string(data))
}

func RemoveKnownUser(userID string) error {
	users := GetKnownUsers()
	filtered := make([]string, 0, len(users))
	for _, u := range users {
		if u != userID {
			filtered = append(filtered, u)
		}
	}
	data, _ := json.Marshal(filtered)
	return keyring.Set(serviceName, knownUsersKey, string(data))
}

func GetKnownUsers() []string {
	raw, err := keyring.Get(serviceName, knownUsersKey)
	if err != nil {
		return nil
	}
	var users []string
	_ = json.Unmarshal([]byte(raw), &users)
	return users
}
