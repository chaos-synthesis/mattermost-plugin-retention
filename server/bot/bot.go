package bot

import (
	"bytes"
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	BotUserName    = "retention-policy"
	BotDisplayName = "Retention Bot"
	BotDescription = "Created by the Retention Plugin."
)

type Bot struct {
	client *pluginapi.Client
	BotID  string
}

func New(client *pluginapi.Client) (*Bot, error) {
	mmBot := &model.Bot{
		Username:    BotUserName,
		DisplayName: BotDisplayName,
		Description: BotDescription,
	}

	botID, err := client.Bot.EnsureBot(mmBot)
	if err != nil {
		return nil, fmt.Errorf("unable to ensure bot: %w", err)
	}

	return &Bot{
		client: client,
		BotID:  botID,
	}, nil
}

func (b *Bot) SendEphemeralPost(channelID string, userID string, msg string) error {
	post := &model.Post{
		UserId:    b.BotID,
		ChannelId: channelID,
		Message:   msg,
	}
	b.client.Post.SendEphemeralPost(userID, post)
	return nil
}

func (b *Bot) SendPostWithAttachment(channelID string, msg string, file *model.FileInfo) error {
	post := &model.Post{
		UserId:    b.BotID,
		ChannelId: channelID,
		Message:   msg,
		FileIds:   []string{file.Id},
	}
	return b.client.Post.CreatePost(post)
}

func (b *Bot) SendDirectPost(userID string, msg string) error {
	channel, err := b.client.Channel.GetDirect(userID, b.BotID)
	if err != nil {
		return fmt.Errorf("bot cannot send direct message: %w", err)
	}

	post := &model.Post{
		UserId:    b.BotID,
		ChannelId: channel.Id,
		Message:   msg,
	}
	return b.client.Post.CreatePost(post)
}

func (b *Bot) SendPost(channelID string, msg string) error {
	post := &model.Post{
		UserId:    b.BotID,
		ChannelId: channelID,
		Message:   msg,
	}
	return b.client.Post.CreatePost(post)
}

func (b *Bot) UploadFile(content *bytes.Buffer, fileName, adminChannel string) (*model.FileInfo, error) {
	return b.client.File.Upload(content, fileName, adminChannel)
}
