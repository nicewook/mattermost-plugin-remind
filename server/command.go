package main

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
)

const defaultCommandTrigger = "remind"

type Configuration struct {
	Trigger string
}

func (p *Plugin) registerCommand() error {
	trigger := p.getCommandTrigger()
	if err := p.API.RegisterCommand(&model.Command{
		Trigger:          trigger,
		AutoComplete:     true,
		AutoCompleteHint: "[@someone or ~channel] [what] [when]",
		AutoCompleteDesc: "Set a reminder",
	}); err != nil {
		return errors.Wrap(err, "failed to register command")
	}

	p.trigger = trigger
	return nil
}

func (p *Plugin) getCommandTrigger() string {
	configuration := &Configuration{
		Trigger: defaultCommandTrigger,
	}

	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		p.API.LogError("failed to load plugin configuration", "err", err.Error())
		return defaultCommandTrigger
	}

	trigger := strings.TrimSpace(configuration.Trigger)
	if trigger == "" || strings.HasPrefix(trigger, "/") || strings.Contains(trigger, " ") {
		p.API.LogError("invalid trigger configured; falling back to default", "trigger", configuration.Trigger)
		return defaultCommandTrigger
	}

	return trigger
}

func (p *Plugin) commandTrigger() string {
	if p.trigger == "" {
		return defaultCommandTrigger
	}
	return p.trigger
}

func (p *Plugin) OnConfigurationChange() error {
	previousTrigger := p.commandTrigger()
	nextTrigger := p.getCommandTrigger()
	if previousTrigger == nextTrigger {
		return nil
	}

	if err := p.API.UnregisterCommand("", previousTrigger); err != nil {
		return errors.Wrap(err, "failed to unregister previous command")
	}

	return p.registerCommand()
}

func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {

	user, uErr := p.API.GetUser(args.UserId)
	if uErr != nil {
		return &model.CommandResponse{}, uErr
	}

	T, locale := p.translation(user)
	location := p.location(user)
	command := strings.Trim(args.Command, " ")
	trigger := p.commandTrigger()

	if strings.Trim(command, " ") == "/"+trigger {
		p.InteractiveSchedule(args.TriggerId, user)
		return &model.CommandResponse{}, nil
	}

	if strings.HasSuffix(command, T("help")) {
		post := model.Post{
			ChannelId: args.ChannelId,
			UserId:    p.botUserId,
			Message:   T("help.response"),
		}
		p.API.SendEphemeralPost(user.Id, &post)
		return &model.CommandResponse{}, nil
	}

	if strings.HasSuffix(command, T("list")) {
		p.API.SendEphemeralPost(user.Id, p.ListReminders(user, args.ChannelId))
		return &model.CommandResponse{}, nil
	}

	// clear all reminders for current user
	if strings.HasSuffix(command, "__clear") {
		post := model.Post{
			ChannelId: args.ChannelId,
			UserId:    p.botUserId,
			Message:   p.DeleteReminders(user),
		}
		p.API.SendEphemeralPost(user.Id, &post)
		return &model.CommandResponse{}, nil
	}

	// display the plugin version
	if strings.HasSuffix(command, "__version") {
		post := model.Post{
			ChannelId: args.ChannelId,
			UserId:    p.botUserId,
			Message:   manifest.Version,
		}
		p.API.SendEphemeralPost(user.Id, &post)
		return &model.CommandResponse{}, nil
	}

	// display the locale & location of user
	if strings.HasSuffix(command, "__user") {
		post := model.Post{
			ChannelId: args.ChannelId,
			UserId:    p.botUserId,
			Message:   "locale: " + locale + "\nlocation: " + location.String(),
		}
		p.API.SendEphemeralPost(user.Id, &post)
		return &model.CommandResponse{}, nil
	}

	payload := strings.Trim(strings.Replace(command, "/"+trigger, "", 1), " ")
	request := ReminderRequest{
		TeamId:   args.TeamId,
		Username: user.Username,
		Payload:  payload,
		Reminder: Reminder{},
	}
	reminder, err := p.ScheduleReminder(&request, args.ChannelId)

	if err != nil {
		post := model.Post{
			ChannelId: args.ChannelId,
			UserId:    p.botUserId,
			Message:   T("exception.response"),
		}
		p.API.SendEphemeralPost(user.Id, &post)
		return &model.CommandResponse{}, nil
	}

	p.API.SendEphemeralPost(user.Id, reminder)
	return &model.CommandResponse{}, nil

}
