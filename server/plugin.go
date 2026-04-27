package main

import (
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/pkg/errors"
)

const (
	botUserName    = "remindbot"
	botDisplayName = "Remindbot"
)

type Plugin struct {
	plugin.MattermostPlugin
	client *pluginapi.Client

	router    *mux.Router
	botUserId string
	trigger   string
	emptyTime time.Time

	schedulerMu   sync.Mutex
	schedulerStop chan struct{}

	ServerConfig *model.Config

	readFile func(path string) ([]byte, error)
	locales  map[string]string
}

func NewPlugin() *Plugin {
	return &Plugin{
		readFile: os.ReadFile,
		locales:  make(map[string]string),
	}
}

func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	p.ServerConfig = p.API.GetConfig()
	p.router = p.InitAPI()
	p.emptyTime = time.Time{}.AddDate(1, 1, 1)

	botID, err := p.client.Bot.EnsureBot(&model.Bot{
		Username:    botUserName,
		DisplayName: botDisplayName,
		Description: "Created by the Flexing Remind plugin.",
	}, pluginapi.ProfileImagePath("assets/icon.png"))
	if err != nil {
		return errors.Wrap(err, "failed to ensure remind bot")
	}
	p.botUserId = botID

	err = p.registerCommand()
	if err != nil {
		return errors.Wrap(err, "failed to register command")
	}

	if err := p.TranslationsPreInit(); err != nil {
		return errors.Wrap(err, "failed to initialize translations")
	}
	p.Run()

	return nil
}

func (p *Plugin) OnDeactivate() error {
	p.Stop()
	return nil
}
