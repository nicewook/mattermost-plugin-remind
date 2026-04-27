package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func (p *Plugin) InitAPI() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/dialog", p.handleDialog).Methods("POST")

	r.HandleFunc("/view/ephemeral", p.handleViewEphemeral).Methods("POST")
	r.HandleFunc("/view/complete/list", p.handleViewCompleteList).Methods("POST")

	r.HandleFunc("/complete", p.handleComplete).Methods("POST")
	r.HandleFunc("/complete/list", p.handleCompleteList).Methods("POST")

	r.HandleFunc("/delete", p.handleDelete).Methods("POST")
	r.HandleFunc("/delete/ephemeral", p.handleDeleteEphemeral).Methods("POST")
	r.HandleFunc("/delete/list", p.handleDeleteList).Methods("POST")
	r.HandleFunc("/delete/complete/list", p.handleDeleteCompleteList).Methods("POST")

	r.HandleFunc("/snooze", p.handleSnooze).Methods("POST")
	r.HandleFunc("/snooze/list", p.handleSnoozeList).Methods("POST")

	r.HandleFunc("/close/list", p.handleCloseList).Methods("POST")

	r.HandleFunc("/next/reminders", p.handleNextReminders).Methods("POST")

	return r
}

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	p.router.ServeHTTP(w, r)
}

func readSubmitDialogRequest(w http.ResponseWriter, req *http.Request) (*model.SubmitDialogRequest, bool) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return nil, false
	}
	defer req.Body.Close()

	var request model.SubmitDialogRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return nil, false
	}

	return &request, true
}

func readPostActionRequest(w http.ResponseWriter, req *http.Request, requiredContext ...string) (*model.PostActionIntegrationRequest, bool) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return nil, false
	}
	defer req.Body.Close()

	var request model.PostActionIntegrationRequest
	if err := json.Unmarshal(body, &request); err != nil {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return nil, false
	}

	for _, key := range requiredContext {
		if _, ok := contextString(request.Context, key); !ok {
			writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
			return nil, false
		}
	}

	return &request, true
}

func contextString(context model.StringInterface, key string) (string, bool) {
	if context == nil {
		return "", false
	}

	value, ok := context[key]
	if !ok {
		return "", false
	}

	result, ok := value.(string)
	if !ok || result == "" {
		return "", false
	}

	return result, true
}

func contextInt(context model.StringInterface, key string) (int, bool) {
	if context == nil {
		return 0, false
	}

	value, ok := context[key]
	if !ok {
		return 0, false
	}

	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func submissionString(submission model.StringInterface, key string) (string, bool) {
	if submission == nil {
		return "", false
	}

	value, ok := submission[key]
	if !ok {
		return "", false
	}

	result, ok := value.(string)
	if !ok {
		return "", false
	}

	return result, true
}

func (p *Plugin) handleDialog(w http.ResponseWriter, req *http.Request) {

	request, ok := readSubmitDialogRequest(w, req)
	if !ok {
		return
	}

	user, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	T, _ := p.translation(user)
	location := p.location(user)

	message, ok := submissionString(request.Submission, "message")
	if !ok || strings.TrimSpace(message) == "" {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	target, ok := submissionString(request.Submission, "target")
	if !ok {
		target = T("me")
	}

	ttime, ok := submissionString(request.Submission, "time")
	if !ok || strings.TrimSpace(ttime) == "" {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	if target == "" {
		target = T("me")
	}
	if target != T("me") &&
		!strings.HasPrefix(target, "@") &&
		!strings.HasPrefix(target, "~") {
		target = "@" + target
	}

	var when string
	if ttime == "unit.test" {
		when = "in 20 minutes"
	} else {
		when = T("in") + " " + T("button.snooze."+ttime)
		switch ttime {
		case "tomorrow":
			when = T("tomorrow")
		case "nextweek":
			when = T("monday")
		}
	}

	r := &ReminderRequest{
		TeamId:   request.TeamId,
		Username: user.Username,
		Payload:  message,
		Reminder: Reminder{
			Id:        model.NewId(),
			TeamId:    request.TeamId,
			Username:  user.Username,
			Message:   message,
			Completed: p.emptyTime,
			Target:    target,
			When:      when,
		},
	}

	if cErr := p.CreateOccurrences(r); cErr != nil {
		p.API.LogError(cErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	if rErr := p.UpsertReminder(r); rErr != nil {
		p.API.LogError(rErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	if r.Reminder.Target == T("me") {
		r.Reminder.Target = T("you")
	}

	useTo := strings.HasPrefix(r.Reminder.Message, T("to"))
	var useToString string
	if useTo {
		useToString = " " + T("to")
	} else {
		useToString = ""
	}

	t := ""
	if len(r.Reminder.Occurrences) > 0 {
		t = r.Reminder.Occurrences[0].Occurrence.In(location).Format(time.RFC3339)
	}
	var responseParameters = map[string]interface{}{
		"Target":  r.Reminder.Target,
		"UseTo":   useToString,
		"Message": r.Reminder.Message,
		"When": p.formatWhen(
			r.Username,
			r.Reminder.When,
			t,
			false,
		),
	}

	reminder := &model.Post{
		ChannelId: request.ChannelId,
		UserId:    p.botUserId,
		Props: model.StringInterface{
			"attachments": []*model.SlackAttachment{
				{
					Text: T("schedule.response", responseParameters),
					Actions: []*model.PostAction{
						{
							Integration: &model.PostActionIntegration{
								Context: model.StringInterface{
									"reminder_id":   r.Reminder.Id,
									"occurrence_id": r.Reminder.Occurrences[0].Id,
									"action":        "delete/ephemeral",
								},
								URL: fmt.Sprintf("/plugins/%s/delete/ephemeral", manifest.ID),
							},
							Type: model.PostActionTypeButton,
							Name: T("button.delete"),
						},
						{
							Integration: &model.PostActionIntegration{
								Context: model.StringInterface{
									"reminder_id":   r.Reminder.Id,
									"occurrence_id": r.Reminder.Occurrences[0].Id,
									"action":        "view/ephemeral",
								},
								URL: fmt.Sprintf("/plugins/%s/view/ephemeral", manifest.ID),
							},
							Type: model.PostActionTypeButton,
							Name: T("button.view.reminders"),
						},
					},
				},
			},
		},
	}
	p.API.SendEphemeralPost(user.Id, reminder)

}

func (p *Plugin) handleViewEphemeral(w http.ResponseWriter, r *http.Request) {

	request, ok := readPostActionRequest(w, r)
	if !ok {
		return
	}

	user, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}
	p.API.SendEphemeralPost(user.Id, p.ListReminders(user, request.ChannelId))

	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})

}

func (p *Plugin) handleComplete(w http.ResponseWriter, r *http.Request) {

	request, ok := readPostActionRequest(w, r, "orig_user_id", "reminder_id", "occurrence_id")
	if !ok {
		return
	}

	origUserID, _ := contextString(request.Context, "orig_user_id")
	reminderID, _ := contextString(request.Context, "reminder_id")

	reminder := p.GetReminder(origUserID, reminderID)
	user, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}
	T, _ := p.translation(user)

	for _, occurrence := range reminder.Occurrences {
		p.ClearScheduledOccurrence(reminder, occurrence)
	}

	reminder.Completed = time.Now().UTC()
	urErr := p.UpdateReminder(origUserID, reminder)
	if urErr != nil {
		p.API.LogError("failed to update reminder %s", urErr)
	}

	if post, pErr := p.API.GetPost(request.PostId); pErr != nil {
		p.API.LogError("unable to get post " + pErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
	} else {

		user, uError := p.API.GetUser(request.UserId)
		if uError != nil {
			p.API.LogError(uError.Error())
			return
		}
		finalTarget := reminder.Target
		if finalTarget == T("me") {
			finalTarget = T("you")
		} else {
			finalTarget = "@" + user.Username
		}

		messageParameters := map[string]interface{}{
			"FinalTarget": finalTarget,
			"Message":     reminder.Message,
		}

		var updateParameters = map[string]interface{}{
			"Message": reminder.Message,
		}

		post.Message = "~~" + T("reminder.message", messageParameters) + "~~\n" + T("action.complete", updateParameters)
		post.Props = model.StringInterface{}
		_, upErr := p.API.UpdatePost(post)
		if upErr != nil {
			p.API.LogError("failed to update post %s", upErr)
		}

		if reminder.Username != user.Username {
			if originalUser, uErr := p.API.GetUserByUsername(reminder.Username); uErr != nil {
				p.API.LogError(uErr.Error())
				writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
				return
			} else {
				if channel, cErr := p.API.GetDirectChannel(p.botUserId, originalUser.Id); cErr != nil {
					p.API.LogError("failed to create channel " + cErr.Error())
					writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
				} else {
					var postbackUpdateParameters = map[string]interface{}{
						"User":    "@" + user.Username,
						"Message": reminder.Message,
					}
					if _, pErr := p.API.CreatePost(&model.Post{
						ChannelId: channel.Id,
						UserId:    p.botUserId,
						Message:   T("action.complete.callback", postbackUpdateParameters),
					}); pErr != nil {
						p.API.LogError(pErr.Error())
						writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
					}
				}
			}
		}

		writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
	}

}

func (p *Plugin) handleDelete(w http.ResponseWriter, r *http.Request) {

	request, ok := readPostActionRequest(w, r, "orig_user_id", "reminder_id", "occurrence_id")
	if !ok {
		return
	}

	origUserID, _ := contextString(request.Context, "orig_user_id")
	reminderID, _ := contextString(request.Context, "reminder_id")

	reminder := p.GetReminder(origUserID, reminderID)
	user, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}
	T, _ := p.translation(user)

	for _, occurrence := range reminder.Occurrences {
		p.ClearScheduledOccurrence(reminder, occurrence)
	}

	message := reminder.Message
	dErr := p.DeleteReminder(origUserID, reminder)
	if dErr != nil {
		p.API.LogError("failed to delete reminder %s", dErr)
	}

	if post, pErr := p.API.GetPost(request.PostId); pErr != nil {
		p.API.LogError(pErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
	} else {
		var deleteParameters = map[string]interface{}{
			"Message": message,
		}
		post.Message = T("action.delete", deleteParameters)
		post.Props = model.StringInterface{}
		_, upErr := p.API.UpdatePost(post)
		if upErr != nil {
			p.API.LogError("failed to update post %s", upErr)
		}
		writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
	}

}

func (p *Plugin) handleDeleteEphemeral(w http.ResponseWriter, r *http.Request) {

	request, ok := readPostActionRequest(w, r, "reminder_id")
	if !ok {
		return
	}

	reminderID, _ := contextString(request.Context, "reminder_id")

	reminder := p.GetReminder(request.UserId, reminderID)
	user, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}
	T, _ := p.translation(user)

	for _, occurrence := range reminder.Occurrences {
		p.ClearScheduledOccurrence(reminder, occurrence)
	}

	message := reminder.Message
	dErr := p.DeleteReminder(request.UserId, reminder)
	if dErr != nil {
		p.API.LogError("failed to delete reminder %s", dErr)
	}
	var deleteParameters = map[string]interface{}{
		"Message": message,
	}
	post := &model.Post{
		Id:        request.PostId,
		UserId:    p.botUserId,
		ChannelId: request.ChannelId,
		Message:   T("action.delete", deleteParameters),
	}
	p.API.UpdateEphemeralPost(request.UserId, post)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})

}

func (p *Plugin) handleSnooze(w http.ResponseWriter, r *http.Request) {

	request, ok := readPostActionRequest(w, r, "orig_user_id", "reminder_id", "occurrence_id", "selected_option")
	if !ok {
		return
	}

	origUserID, _ := contextString(request.Context, "orig_user_id")
	reminderID, _ := contextString(request.Context, "reminder_id")
	occurrenceID, _ := contextString(request.Context, "occurrence_id")
	selectedOption, _ := contextString(request.Context, "selected_option")

	reminder := p.GetReminder(origUserID, reminderID)
	user, uErr := p.API.GetUser(request.UserId)
	if uErr != nil {
		p.API.LogError(uErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}
	T, _ := p.translation(user)

	for _, occurrence := range reminder.Occurrences {
		if occurrence.Id == occurrenceID {
			p.ClearScheduledOccurrence(reminder, occurrence)
		}
	}

	if post, pErr := p.API.GetPost(request.PostId); pErr != nil {
		p.API.LogError("unable to get post " + pErr.Error())
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
	} else {
		var snoozeParameters = map[string]interface{}{
			"Message": reminder.Message,
		}

		switch selectedOption {
		case "20min":
			for i, occurrence := range reminder.Occurrences {
				if occurrence.Id == occurrenceID {
					occurrence.Snoozed = time.Now().UTC().Round(time.Second).Add(time.Minute * time.Duration(20))
					reminder.Occurrences[i] = occurrence
					upErr := p.UpdateReminder(origUserID, reminder)
					if upErr != nil {
						p.API.LogError("failed to update reminder %s", upErr)
					}
					p.upsertSnoozedOccurrence(&occurrence)
					post.Message = T("action.snooze.20min", snoozeParameters)
					break
				}
			}
		case "1hr":
			for i, occurrence := range reminder.Occurrences {
				if occurrence.Id == occurrenceID {
					occurrence.Snoozed = time.Now().UTC().Round(time.Second).Add(time.Hour * time.Duration(1))
					reminder.Occurrences[i] = occurrence
					upErr := p.UpdateReminder(origUserID, reminder)
					if upErr != nil {
						p.API.LogError("failed to update reminder %s", upErr)
					}
					p.upsertSnoozedOccurrence(&occurrence)
					post.Message = T("action.snooze.1hr", snoozeParameters)
					break
				}
			}
		case "3hrs":
			for i, occurrence := range reminder.Occurrences {
				if occurrence.Id == occurrenceID {
					occurrence.Snoozed = time.Now().UTC().Round(time.Second).Add(time.Hour * time.Duration(3))
					reminder.Occurrences[i] = occurrence
					upErr := p.UpdateReminder(origUserID, reminder)
					if upErr != nil {
						p.API.LogError("failed to update reminder %s", upErr)
					}
					p.upsertSnoozedOccurrence(&occurrence)
					post.Message = T("action.snooze.3hr", snoozeParameters)
					break
				}
			}
		case "tomorrow":
			for i, occurrence := range reminder.Occurrences {
				if occurrence.Id == occurrenceID {

					if user, uErr := p.API.GetUser(request.UserId); uErr != nil {
						p.API.LogError(uErr.Error())
						return
					} else {
						location := p.location(user)
						tt := time.Now().In(location).Add(time.Hour * time.Duration(24))
						occurrence.Snoozed = time.Date(tt.Year(), tt.Month(), tt.Day(), 9, 0, 0, 0, location).UTC()
						reminder.Occurrences[i] = occurrence
						upErr := p.UpdateReminder(origUserID, reminder)
						if upErr != nil {
							p.API.LogError("failed to update reminder %s", upErr)
						}
						p.upsertSnoozedOccurrence(&occurrence)
						post.Message = T("action.snooze.tomorrow", snoozeParameters)
						break
					}
				}
			}
		case "nextweek":
			for i, occurrence := range reminder.Occurrences {
				if occurrence.Id == occurrenceID {

					if user, uErr := p.API.GetUser(request.UserId); uErr != nil {
						p.API.LogError(uErr.Error())
						return
					} else {
						location := p.location(user)

						todayWeekDayNum := int(time.Now().In(location).Weekday())
						weekDayNum := 1
						day := 0

						if weekDayNum < todayWeekDayNum {
							day = 7 - (todayWeekDayNum - weekDayNum)
						} else if weekDayNum >= todayWeekDayNum {
							day = 7 + (weekDayNum - todayWeekDayNum)
						}

						tt := time.Now().In(location).Add(time.Hour * time.Duration(24))
						occurrence.Snoozed = time.Date(tt.Year(), tt.Month(), tt.Day(), 9, 0, 0, 0, location).AddDate(0, 0, day).UTC()
						reminder.Occurrences[i] = occurrence
						upErr := p.UpdateReminder(origUserID, reminder)
						if upErr != nil {
							p.API.LogError("failed to update reminder %s", upErr)
						}
						p.upsertSnoozedOccurrence(&occurrence)
						post.Message = T("action.snooze.nextweek", snoozeParameters)
						break
					}
				}
			}
		}

		post.Props = model.StringInterface{}
		_, upErr := p.API.UpdatePost(post)
		if upErr != nil {
			p.API.LogError("failed to update post %s", upErr)
		}
		writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
	}
}

func (p *Plugin) handleNextReminders(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r)
	if !ok {
		return
	}

	offset, ok := contextInt(request.Context, "offset")
	if !ok {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	p.UpdateListReminders(request.UserId, request.PostId, request.ChannelId, offset)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func (p *Plugin) handleCompleteList(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r, "reminder_id")
	if !ok {
		return
	}

	reminderID, _ := contextString(request.Context, "reminder_id")
	offset, ok := contextInt(request.Context, "offset")
	if !ok {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	reminder := p.GetReminder(request.UserId, reminderID)

	for _, occurrence := range reminder.Occurrences {
		p.ClearScheduledOccurrence(reminder, occurrence)
	}

	reminder.Completed = time.Now().UTC()
	upErr := p.UpdateReminder(request.UserId, reminder)
	if upErr != nil {
		p.API.LogError("failed to update reminder %s", upErr)
	}
	p.UpdateListReminders(request.UserId, request.PostId, request.ChannelId, offset)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func (p *Plugin) handleViewCompleteList(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r)
	if !ok {
		return
	}

	p.ListCompletedReminders(request.UserId, request.PostId, request.ChannelId)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func (p *Plugin) handleDeleteList(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r, "reminder_id")
	if !ok {
		return
	}

	reminderID, _ := contextString(request.Context, "reminder_id")
	offset, ok := contextInt(request.Context, "offset")
	if !ok {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	reminder := p.GetReminder(request.UserId, reminderID)

	for _, occurrence := range reminder.Occurrences {
		p.ClearScheduledOccurrence(reminder, occurrence)
	}

	dErr := p.DeleteReminder(request.UserId, reminder)
	if dErr != nil {
		p.API.LogError("failed to update post %s", dErr)
	}
	p.UpdateListReminders(request.UserId, request.PostId, request.ChannelId, offset)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func (p *Plugin) handleDeleteCompleteList(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r)
	if !ok {
		return
	}

	offset, ok := contextInt(request.Context, "offset")
	if !ok {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	p.DeleteCompletedReminders(request.UserId)
	p.UpdateListReminders(request.UserId, request.PostId, request.ChannelId, offset)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func (p *Plugin) handleSnoozeList(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r, "reminder_id", "occurrence_id", "selected_option")
	if !ok {
		return
	}

	reminderID, _ := contextString(request.Context, "reminder_id")
	occurrenceID, _ := contextString(request.Context, "occurrence_id")
	selectedOption, _ := contextString(request.Context, "selected_option")
	offset, ok := contextInt(request.Context, "offset")
	if !ok {
		writePostActionIntegrationResponseError(w, &model.PostActionIntegrationResponse{})
		return
	}

	reminder := p.GetReminder(request.UserId, reminderID)

	for _, occurrence := range reminder.Occurrences {
		if occurrence.Id == occurrenceID {
			p.ClearScheduledOccurrence(reminder, occurrence)
		}
	}

	switch selectedOption {
	case "20min":
		for i, occurrence := range reminder.Occurrences {
			if occurrence.Id == occurrenceID {
				occurrence.Snoozed = time.Now().UTC().Round(time.Second).Add(time.Minute * time.Duration(20))
				reminder.Occurrences[i] = occurrence
				upErr := p.UpdateReminder(request.UserId, reminder)
				if upErr != nil {
					p.API.LogError("failed to update reminder %s", upErr)
				}
				p.upsertSnoozedOccurrence(&occurrence)
				break
			}
		}
	case "1hr":
		for i, occurrence := range reminder.Occurrences {
			if occurrence.Id == occurrenceID {
				occurrence.Snoozed = time.Now().UTC().Round(time.Second).Add(time.Hour * time.Duration(1))
				reminder.Occurrences[i] = occurrence
				upErr := p.UpdateReminder(request.UserId, reminder)
				if upErr != nil {
					p.API.LogError("failed to update reminder %s", upErr)
				}
				p.upsertSnoozedOccurrence(&occurrence)
				break
			}
		}
	case "3hrs":
		for i, occurrence := range reminder.Occurrences {
			if occurrence.Id == occurrenceID {
				occurrence.Snoozed = time.Now().UTC().Round(time.Second).Add(time.Hour * time.Duration(3))
				reminder.Occurrences[i] = occurrence
				upErr := p.UpdateReminder(request.UserId, reminder)
				if upErr != nil {
					p.API.LogError("failed to update reminder %s", upErr)
				}
				p.upsertSnoozedOccurrence(&occurrence)
				break
			}
		}
	case "tomorrow":
		for i, occurrence := range reminder.Occurrences {
			if occurrence.Id == occurrenceID {

				if user, uErr := p.API.GetUser(request.UserId); uErr != nil {
					p.API.LogError(uErr.Error())
					return
				} else {
					location := p.location(user)
					tt := time.Now().In(location).Add(time.Hour * time.Duration(24))
					occurrence.Snoozed = time.Date(tt.Year(), tt.Month(), tt.Day(), 9, 0, 0, 0, location).UTC()
					reminder.Occurrences[i] = occurrence
					upErr := p.UpdateReminder(request.UserId, reminder)
					if upErr != nil {
						p.API.LogError("failed to update reminder %s", upErr)
					}
					p.upsertSnoozedOccurrence(&occurrence)
					break
				}
			}
		}
	case "nextweek":
		for i, occurrence := range reminder.Occurrences {
			if occurrence.Id == occurrenceID {

				if user, uErr := p.API.GetUser(request.UserId); uErr != nil {
					p.API.LogError(uErr.Error())
					return
				} else {
					location := p.location(user)

					todayWeekDayNum := int(time.Now().In(location).Weekday())
					weekDayNum := 1
					day := 0

					if weekDayNum < todayWeekDayNum {
						day = 7 - (todayWeekDayNum - weekDayNum)
					} else if weekDayNum >= todayWeekDayNum {
						day = 7 + (weekDayNum - todayWeekDayNum)
					}

					tt := time.Now().In(location).Add(time.Hour * time.Duration(24))
					occurrence.Snoozed = time.Date(tt.Year(), tt.Month(), tt.Day(), 9, 0, 0, 0, location).AddDate(0, 0, day).UTC()
					reminder.Occurrences[i] = occurrence
					upErr := p.UpdateReminder(request.UserId, reminder)
					if upErr != nil {
						p.API.LogError("failed to update reminder %s", upErr)
					}
					p.upsertSnoozedOccurrence(&occurrence)
					break
				}
			}
		}
	}

	p.UpdateListReminders(request.UserId, request.PostId, request.ChannelId, offset)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func (p *Plugin) handleCloseList(w http.ResponseWriter, r *http.Request) {
	request, ok := readPostActionRequest(w, r)
	if !ok {
		return
	}

	p.API.DeleteEphemeralPost(request.UserId, request.PostId)
	writePostActionIntegrationResponseOk(w, &model.PostActionIntegrationResponse{})
}

func writePostActionIntegrationResponseOk(w http.ResponseWriter, response *model.PostActionIntegrationResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	responseJSON, _ := json.Marshal(response)
	_, _ = w.Write(responseJSON)
}

func writePostActionIntegrationResponseError(w http.ResponseWriter, response *model.PostActionIntegrationResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	responseJSON, _ := json.Marshal(response)
	_, _ = w.Write(responseJSON)
}
