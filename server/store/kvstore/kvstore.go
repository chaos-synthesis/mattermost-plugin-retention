package kvstore

import "github.com/mattermost/mattermost/server/public/model"

type UserSettings struct {
	UserID        string
	Enabled       bool
	PostAgeInDays float64
}

// KVStore Define your methods here. This package is used to access the KVStore pluginapi methods.
type KVStore interface {
	GetManifest() *model.Manifest

	GetUserSettings(userID string) (UserSettings, error)

	SetUserSettings(userID string, value *UserSettings) error

	GetActiveUsers() ([]string, error)
}
