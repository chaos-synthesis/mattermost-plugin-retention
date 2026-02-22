package command

import (
	"fmt"
	"strings"

	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store/kvstore"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type Handler struct {
	client *pluginapi.Client
	// kvStore is the client used to read/write KV records for this plugin.
	kvStore kvstore.KVStore
	// botUser used for messaging
	//botUser *rbot.Bot
}

type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	executeCommandInteractive(args *model.CommandArgs) *model.CommandResponse
}

const postRetentionCommandTrigger = "post-retention"

// NewCommandHandler Register all your slash commands.
func NewCommandHandler(client *pluginapi.Client, kvStore kvstore.KVStore) Command {
	err := client.SlashCommand.Register(&model.Command{
		Trigger:          postRetentionCommandTrigger,
		AutoComplete:     true,
		AutoCompleteHint: "",
		AutoCompleteDesc: "Post retention management.",
	})
	if err != nil {
		client.Log.Error("Failed to register command", "error", err)
	}

	return &Handler{
		client:  client,
		kvStore: kvStore,
	}
}

// Handle ExecuteCommand hook calls this method to execute the commands that were registered in the NewCommandHandler function.
func (c *Handler) Handle(args *model.CommandArgs) (*model.CommandResponse, error) {
	fields := strings.Fields(args.Command)
	if len(fields) == 0 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Empty command",
		}, nil
	}
	trigger := strings.TrimPrefix(fields[0], "/")
	switch trigger {
	case postRetentionCommandTrigger:
		return c.executeCommandInteractive(args), nil
	default:
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         fmt.Sprintf("Unknown command: %s", args.Command),
		}, nil
	}
}

func (c *Handler) executeCommandInteractive(args *model.CommandArgs) *model.CommandResponse {
	userSettings, err := c.kvStore.GetUserSettings(args.UserId)
	if err != nil {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			ChannelId:    args.ChannelId,
			Text:         fmt.Sprintf("Failed to get user settings: %s. Please contact administrator", err.Error()),
		}
	}

	post := CreateStateMessagePost(userSettings, fmt.Sprintf("/plugins/%s", c.kvStore.GetManifest().Id), "")

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		ChannelId:    args.ChannelId,
		Attachments:  post.Attachments(),
	}
}

func CreateStateMessagePost(userSettings kvstore.UserSettings, bundleUrl string, message string) *model.Post {
	statusValue := "Inactive"
	if userSettings.Enabled {
		statusValue = "Active"
	}

	postAgeInDaysValue := "N/A"
	if userSettings.Enabled && userSettings.PostAgeInDays > 0 {
		postAgeInDaysValue = fmt.Sprintf("%d days", int(userSettings.PostAgeInDays))
	}

	post := &model.Post{
		Type: model.PostTypeEphemeral,
	}
	post.SetProps(model.StringInterface{
		"attachments": []*model.SlackAttachment{{
			Title: "Posts retention policy",
			Text:  message,
			Fields: []*model.SlackAttachmentField{
				{
					Title: "Status",
					Value: statusValue,
					Short: true,
				},
				{
					Title: "Remove posts after",
					Value: postAgeInDaysValue,
					Short: true,
				},
			},
			Actions: []*model.PostAction{{
				Integration: &model.PostActionIntegration{
					URL: fmt.Sprintf("%s/api/v1/actions/settings", bundleUrl),
				},
				Type: model.PostActionTypeButton,
				Name: "Settings",
			}},
		}},
	})

	return post
}
