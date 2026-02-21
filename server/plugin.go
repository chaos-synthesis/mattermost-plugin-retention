package main

import (
	"net/http"
	"sync"
	"time"

	rbot "github.com/chaos-synthesis/mattermost-plugin-retention/server/bot"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/command"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/config"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/mmctl/commands"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store"
	"github.com/chaos-synthesis/mattermost-plugin-retention/server/store/kvstore"
	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// kvStore is the client used to read/write KV records for this plugin.
	kvStore kvstore.KVStore

	// client is the Mattermost server API client.
	client *pluginapi.Client

	// commandClient is the client used to register and execute slash commands.
	commandClient command.Command

	// router is the HTTP router for handling API requests.
	router *mux.Router

	// botUser used for messaging
	botUser *rbot.Bot

	backgroundJob       *cluster.Job
	backgroundJobHelper PostRetentionJobHelper

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *config.Configuration

	sqlStore *store.SQLStore
}

// OnActivate is invoked when the plugin is activated. If an error is returned, the plugin will be deactivated.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	kvStore, err := kvstore.NewKVStore(p.client, manifest)
	if err != nil {
		return errors.Wrap(err, "failed to create KVStore")
	}
	p.kvStore = kvStore

	p.router = p.initRouter()

	bot, err := rbot.New(p.client)
	if err != nil {
		return errors.Wrap(err, "failed to create bot user")
	}
	p.botUser = bot

	p.commandClient = command.NewCommandHandler(p.client, p.kvStore, bot)

	// Create job for post retention
	commands.PrepareRun()
	p.backgroundJobHelper.plugin = p
	if err := p.backgroundJobHelper.Start(); err != nil {
		return errors.Wrap(err, "failed to schedule background job")
	}

	// Initialize SQL store
	sqlStore, err := store.New(p.client.Store, &p.client.Log)
	if err != nil {
		return errors.Wrap(err, "failed to create SQLStore")
	}
	p.sqlStore = sqlStore

	return nil
}

// OnDeactivate is invoked when the plugin is deactivated.
func (p *Plugin) OnDeactivate() error {
	if err := p.backgroundJobHelper.Stop(time.Second * 15); err != nil {
		p.API.LogError("Failed to close background job(helper)", "err", err)
	}

	if p.backgroundJob != nil {
		if err := p.backgroundJob.Close(); err != nil {
			p.API.LogError("Failed to close background job", "err", err)
		}
	}

	return nil
}

// ExecuteCommand This will execute the commands that were registered in the NewCommandHandler function.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	response, err := p.commandClient.Handle(args)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return response, nil
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
