package kvstore

import (
	"slices"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"
)

const (
	activeUsersKeyPrefix  = "rpp_active_users"
	userSettingsKeyPrefix = "rpp_user_settings-"
)

// We expose our calls to the KVStore pluginapi methods through this interface for testability and stability.
// This allows us to better control which values are stored with which keys.

type StoreImpl struct {
	client   *pluginapi.Client
	manifest *model.Manifest
}

func NewKVStore(client *pluginapi.Client, manifest *model.Manifest) (StoreImpl, error) {
	var activeUsers []string
	err := client.KV.Get(activeUsersKeyPrefix, &activeUsers)
	if err != nil {
		return StoreImpl{}, errors.Wrap(err, "failed to get active users")
	}

	return StoreImpl{
		client:   client,
		manifest: manifest,
	}, nil
}

func (kv StoreImpl) GetManifest() *model.Manifest {
	return kv.manifest
}

// GetTemplateData Sample method to get a key-value pair in the KV store
func (kv StoreImpl) GetTemplateData(userID string) (string, error) {
	var templateData string
	err := kv.client.KV.Get("rpp_template_key-"+userID, &templateData)
	if err != nil {
		return "", errors.Wrap(err, "failed to get template data")
	}
	return templateData, nil
}

func (kv StoreImpl) GetUserSettings(userID string) (UserSettings, error) {
	var userSettings UserSettings
	err := kv.client.KV.Get(userSettingsKeyPrefix+userID, &userSettings)
	if err != nil {
		return UserSettings{}, errors.Wrap(err, "failed to get user settings")
	}
	return userSettings, nil
}

func (kv StoreImpl) SetUserSettings(userID string, value *UserSettings) error {
	if value.Enabled {
		_, err := kv.addActiveUser(userID)
		if err != nil {
			return errors.Wrap(err, "failed to add active user settings")
		}
	} else {
		_, err := kv.removeActiveUser(userID)
		if err != nil {
			return errors.Wrap(err, "failed to remove active user settings")
		}
	}

	_, err := kv.client.KV.Set(userSettingsKeyPrefix+userID, value)
	if err != nil {
		return errors.Wrap(err, "failed to set user settings")
	}
	return nil
}

func (kv StoreImpl) GetActiveUsers() ([]string, error) {
	var activeUsers []string
	err := kv.client.KV.Get(activeUsersKeyPrefix, &activeUsers)
	if err != nil {
		return []string{}, errors.Wrap(err, "failed to get active users")
	}
	return activeUsers, nil
}

func (kv StoreImpl) addActiveUser(userID string) (bool, error) {
	activeUsers, err := kv.GetActiveUsers()
	if err != nil {
		return false, err
	}
	if slices.Contains(activeUsers, userID) {
		return false, nil
	}

	activeUsers = append(activeUsers, userID)

	return kv.client.KV.Set(activeUsersKeyPrefix, activeUsers)
}

func (kv StoreImpl) removeActiveUser(userID string) (bool, error) {
	activeUsers, err := kv.GetActiveUsers()
	if err != nil {
		return false, err
	}
	if !slices.Contains(activeUsers, userID) {
		return false, nil
	}

	idx := slices.Index(activeUsers, userID)
	activeUsers = slices.Delete(activeUsers, idx, idx+1)
	return kv.client.KV.Set(activeUsersKeyPrefix, activeUsers)
}
