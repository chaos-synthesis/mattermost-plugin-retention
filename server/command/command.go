package command

import (
	"fmt"
	"strings"

	rbot "github.com/chaos-synthesis/mattermost-plugin-retention/server/bot"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store/kvstore"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

type Handler struct {
	client *pluginapi.Client
	// kvStore is the client used to read/write KV records for this plugin.
	kvStore kvstore.KVStore
	// botUser used for messaging
	botUser *rbot.Bot
}

type Command interface {
	Handle(args *model.CommandArgs) (*model.CommandResponse, error)
	executeHelloCommand(args *model.CommandArgs) *model.CommandResponse
}

const postRetentionCommandTrigger = "post-retention"

// NewCommandHandler Register all your slash commands.
func NewCommandHandler(client *pluginapi.Client, kvStore kvstore.KVStore, botUser *rbot.Bot) Command {
	err := client.SlashCommand.Register(&model.Command{
		Trigger:          postRetentionCommandTrigger,
		AutoComplete:     true,
		AutoCompleteHint: "",
		AutoCompleteDesc: "Post retention policy management.",
	})
	if err != nil {
		client.Log.Error("Failed to register command", "error", err)
	}

	return &Handler{
		client:  client,
		kvStore: kvStore,
		botUser: botUser,
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

func (c *Handler) executeHelloCommand(args *model.CommandArgs) *model.CommandResponse {
	if len(strings.Fields(args.Command)) < 2 {
		return &model.CommandResponse{
			ResponseType: model.CommandResponseTypeEphemeral,
			Text:         "Please specify a username",
		}
	}
	username := strings.Fields(args.Command)[1]
	return &model.CommandResponse{
		Text: "Hello, " + username,
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

	statusValue := "Inactive"
	if userSettings.Enabled {
		statusValue = "Active"
	}

	postAgeInDaysValue := "N/A"
	if userSettings.Enabled && userSettings.PostAgeInDays > 0 {
		postAgeInDaysValue = fmt.Sprintf("%f days", userSettings.PostAgeInDays)
	}

	return &model.CommandResponse{
		ResponseType: model.CommandResponseTypeEphemeral,
		ChannelId:    args.ChannelId,
		Attachments: []*model.SlackAttachment{{
			Title: "Posts retention policy management",
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
					URL: "/api/v1/settings",
					//URL: fmt.Sprintf("/plugins/%s/interactive/button/1", c.kvStore.GetManifest().Id),
				},
				Type: model.PostActionTypeButton,
				Name: "Settings",
			}},
		}},
	}
}
