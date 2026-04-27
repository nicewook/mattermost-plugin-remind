package main

import (
	"fmt"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHelpExamplesSchedule(t *testing.T) {
	user := &model.User{
		Id:       "userID1",
		Email:    "-@-.-",
		Nickname: "TestUser",
		Password: model.NewId(),
		Username: "testuser",
		Roles:    model.SystemUserRoleId,
		Locale:   "en",
	}

	tests := []struct {
		name              string
		command           string
		expectedTarget    string
		expectedMessage   string
		expectedWhen      string
		expectedOccurence int
	}{
		{
			name:              "me in seconds",
			command:           "/remind me plugin test in 10 seconds",
			expectedTarget:    "me",
			expectedMessage:   "plugin test",
			expectedWhen:      "in 10 seconds",
			expectedOccurence: 1,
		},
		{
			name:              "me tomorrow",
			command:           "/remind me submit the report tomorrow at 9am",
			expectedTarget:    "me",
			expectedMessage:   "submit the report",
			expectedWhen:      "tomorrow at 9am",
			expectedOccurence: 1,
		},
		{
			name:              "user in minutes",
			command:           "/remind @jessica client meeting in 30 minutes",
			expectedTarget:    "@jessica",
			expectedMessage:   "client meeting",
			expectedWhen:      "in 30 minutes",
			expectedOccurence: 1,
		},
		{
			name:              "time in hours",
			command:           "/remind me \"time example\" in 2 hours",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "in 2 hours",
			expectedOccurence: 1,
		},
		{
			name:              "time in days",
			command:           "/remind me \"time example\" in 3 days",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "in 3 days",
			expectedOccurence: 1,
		},
		{
			name:              "time at hour",
			command:           "/remind me \"time example\" at 1pm",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "at 1pm",
			expectedOccurence: 1,
		},
		{
			name:              "time at minute",
			command:           "/remind me \"time example\" at 5:30pm",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "at 5:30pm",
			expectedOccurence: 1,
		},
		{
			name:              "time at noon",
			command:           "/remind me \"time example\" at noon",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "at noon",
			expectedOccurence: 1,
		},
		{
			name:              "time at midnight",
			command:           "/remind me \"time example\" at midnight",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "at midnight",
			expectedOccurence: 1,
		},
		{
			name:              "time on monday",
			command:           "/remind me \"time example\" on monday at 10am",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "on monday at 10am",
			expectedOccurence: 1,
		},
		{
			name:              "time on month day",
			command:           "/remind me \"time example\" on June 1st at 8am",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "on June 1st at 8am",
			expectedOccurence: 1,
		},
		{
			name:              "time every monday",
			command:           "/remind me \"time example\" every monday at 10am",
			expectedTarget:    "me",
			expectedMessage:   "time example",
			expectedWhen:      "every monday at 10am",
			expectedOccurence: 1,
		},
		{
			name:              "channel in seconds",
			command:           "/remind ~aicenter \"plugin test\" in 10 seconds",
			expectedTarget:    "~aicenter",
			expectedMessage:   "plugin test",
			expectedWhen:      "in 10 seconds",
			expectedOccurence: 1,
		},
		{
			name:              "channel weekdays",
			command:           "/remind ~aicenter \":rocket: 오늘 할 일을 공유해 주세요! (To-do)\" every weekday at 9am",
			expectedTarget:    "~aicenter",
			expectedMessage:   ":rocket: 오늘 할 일을 공유해 주세요! (To-do)",
			expectedWhen:      "every weekday at 9am",
			expectedOccurence: 5,
		},
		{
			name:              "channel multiple weekdays",
			command:           "/remind ~aicenter \":white_check_mark: 오늘 한 일을 공유해 주세요! (Done)\" every monday, tuesday, wednesday, thursday at 5:30pm",
			expectedTarget:    "~aicenter",
			expectedMessage:   ":white_check_mark: 오늘 한 일을 공유해 주세요! (Done)",
			expectedWhen:      "every monday, tuesday, wednesday, thursday at 5:30pm",
			expectedOccurence: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &plugintest.API{}
			api.On("LogDebug", mock.Anything, mock.Anything, mock.Anything).Maybe()
			api.On("LogError", mock.Anything, mock.Anything, mock.Anything).Maybe()
			api.On("LogInfo", mock.Anything).Maybe()
			api.On("GetUser", user.Id).Return(user, nil)
			api.On("GetUserByUsername", mock.AnythingOfType("string")).Return(user, nil)
			api.On("KVGet", mock.AnythingOfType("string")).Return([]byte("[]"), nil)
			api.On("KVSet", mock.AnythingOfType("string"), mock.Anything).Return(nil)
			api.On("SendEphemeralPost", user.Id, mock.AnythingOfType("*model.Post")).Return(&model.Post{})
			defer api.AssertExpectations(t)

			p := &Plugin{}
			p.API = api
			p.trigger = defaultCommandTrigger
			p.router = p.InitAPI()

			response, err := p.ExecuteCommand(nil, &model.CommandArgs{
				ChannelId: "channelID1",
				Command:   tt.command,
				TeamId:    "teamID1",
				UserId:    user.Id,
			})

			require.Nil(t, err)
			require.NotNil(t, response)

			payload := tt.command[len(fmt.Sprintf("/%s ", defaultCommandTrigger)):]
			request := &ReminderRequest{
				TeamId:   "teamID1",
				Username: user.Username,
				Payload:  payload,
				Reminder: Reminder{},
			}

			require.NoError(t, p.ParseRequest(request))
			require.Equal(t, tt.expectedTarget, request.Reminder.Target)
			require.Equal(t, tt.expectedMessage, request.Reminder.Message)
			require.Equal(t, tt.expectedWhen, request.Reminder.When)
			require.NoError(t, p.CreateOccurrences(request))
			require.Len(t, request.Reminder.Occurrences, tt.expectedOccurence)
		})
	}
}
