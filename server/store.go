package main

import (
	"encoding/json"
	"fmt"
	"time"
)

func reminderStoreKey(username string) string {
	return username
}

func occurrenceStoreKey(t time.Time) string {
	return fmt.Sprintf("%v", t)
}

func (p *Plugin) loadRemindersForUsername(username string) ([]Reminder, error) {
	bytes, err := p.API.KVGet(reminderStoreKey(username))
	if err != nil {
		return nil, err
	}
	if len(bytes) == 0 {
		return []Reminder{}, nil
	}

	var reminders []Reminder
	if err := json.Unmarshal(bytes, &reminders); err != nil {
		return nil, err
	}

	return reminders, nil
}

func (p *Plugin) storeRemindersForUsername(username string, reminders []Reminder) error {
	bytes, err := json.Marshal(reminders)
	if err != nil {
		return err
	}

	if appErr := p.API.KVSet(reminderStoreKey(username), bytes); appErr != nil {
		return appErr
	}

	return nil
}

func (p *Plugin) deleteRemindersForUsername(username string) error {
	if appErr := p.API.KVDelete(reminderStoreKey(username)); appErr != nil {
		return appErr
	}

	return nil
}

func (p *Plugin) loadOccurrencesAt(t time.Time) ([]Occurrence, error) {
	bytes, err := p.API.KVGet(occurrenceStoreKey(t))
	if err != nil {
		return nil, err
	}
	if len(bytes) == 0 {
		return []Occurrence{}, nil
	}

	var occurrences []Occurrence
	if err := json.Unmarshal(bytes, &occurrences); err != nil {
		return nil, err
	}

	return occurrences, nil
}

func (p *Plugin) storeOccurrencesAt(t time.Time, occurrences []Occurrence) error {
	bytes, err := json.Marshal(occurrences)
	if err != nil {
		return err
	}

	if appErr := p.API.KVSet(occurrenceStoreKey(t), bytes); appErr != nil {
		return appErr
	}

	return nil
}
