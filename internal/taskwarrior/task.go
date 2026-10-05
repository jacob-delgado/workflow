// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package taskwarrior

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// Status is a task's state as Taskwarrior names it.
type Status string

// The states a task can be in.
const (
	Pending   Status = "pending"
	Completed Status = "completed"
	Deleted   Status = "deleted"
	Waiting   Status = "waiting"
	Recurring Status = "recurring"
)

// Annotation is a note on a task, and when it was written.
type Annotation struct {
	Entry       time.Time
	Description string
}

// Task is one task as export writes it. Every text field has been through
// sanitize.Line; a synced replica is as untrusted as a file on disk.
type Task struct {
	UUID                        string
	ID                          int // 0 for a task not in the working set
	Description                 string
	Status                      Status
	Project                     string
	Priority                    string // "H", "M", "L" or ""
	Tags                        []string
	Due, Wait, Scheduled, Until time.Time
	Entry, Modified, Start, End time.Time
	Urgency                     float64
	Annotations                 []Annotation
	Depends                     []string
	IssueKey                    string // the jiraid UDA
	IssueURL                    string // the jiraurl UDA
}

// Active reports a pending task that has been started.
func (t Task) Active() bool {
	return t.Status == Pending && !t.Start.IsZero()
}

// Waiting reports a task hidden until a date still after now.
func (t Task) Waiting(now time.Time) bool {
	return t.Status == Waiting || (t.Status == Pending && t.Wait.After(now))
}

// Linked reports a task that names a Jira issue.
func (t Task) Linked() bool {
	return t.IssueKey != ""
}

// IssueLink is what a task created from an issue carries.
type IssueLink struct {
	Key, Summary, URL, Priority string
}

// PriorityFor maps a Jira priority name onto Taskwarrior's H, M, L, or "" for
// a name it does not know — bugwarrior's own map, matched without case.
func PriorityFor(jiraPriority string) string {
	switch strings.ToLower(jiraPriority) {
	case "highest", "high", "critical", "blocker":
		return "H"
	case "medium", "major":
		return "M"
	case "low", "lowest", "minor", "trivial":
		return "L"
	default:
		return ""
	}
}

// TrackLine is the add line for an issue, in Taskwarrior's grammar:
//
//	jiraid:PROJ-42 jiraurl:https://… +jira priority:H -- PROJ-42: <summary>
//
// (priority left out when PriorityFor gives ""). The summary is one line
// (sanitize.Line) and follows --, so a due:tomorrow in it stays words.
func TrackLine(issue IssueLink) string {
	words := []string{LinkUDA + ":" + issue.Key, LinkURLUDA + ":" + issue.URL, "+" + LinkTag}

	if priority := PriorityFor(issue.Priority); priority != "" {
		words = append(words, "priority:"+priority)
	}

	words = append(words, "--", issue.Key+":", sanitize.Line(issue.Summary))

	return strings.Join(words, " ")
}

// dateLayout is how export writes a date: UTC, to the second.
const dateLayout = "20060102T150405Z"

// wireTask is one task as export writes it, before its dates are read and its
// text is made safe to show. A key it does not name — bugwarrior's jirasummary
// and the rest — is ignored.
type wireTask struct {
	UUID        string           `json:"uuid"`
	ID          int              `json:"id"`
	Description string           `json:"description"`
	Status      string           `json:"status"`
	Project     string           `json:"project"`
	Priority    string           `json:"priority"`
	Tags        []string         `json:"tags"`
	Due         string           `json:"due"`
	Wait        string           `json:"wait"`
	Scheduled   string           `json:"scheduled"`
	Until       string           `json:"until"`
	Entry       string           `json:"entry"`
	Modified    string           `json:"modified"`
	Start       string           `json:"start"`
	End         string           `json:"end"`
	Urgency     float64          `json:"urgency"`
	Annotations []wireAnnotation `json:"annotations"`
	Depends     json.RawMessage  `json:"depends"`
	IssueKey    string           `json:"jiraid"`
	IssueURL    string           `json:"jiraurl"`
}

// wireAnnotation is one annotation as export writes it.
type wireAnnotation struct {
	Entry       string `json:"entry"`
	Description string `json:"description"`
}

// decodeTasks reads every task an export wrote.
func decodeTasks(wire []wireTask) ([]Task, error) {
	tasks := make([]Task, 0, len(wire))

	for _, one := range wire {
		task, err := one.task()
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// task reads one wire task: its dates parsed, its text on one line.
func (w wireTask) task() (Task, error) {
	annotations, err := decodeAnnotations(w.Annotations)
	if err != nil {
		return Task{}, err
	}

	depends, err := decodeDepends(w.Depends)
	if err != nil {
		return Task{}, err
	}

	task := Task{
		UUID: sanitize.Line(w.UUID), ID: w.ID, Description: sanitize.Line(w.Description),
		Status: Status(sanitize.Line(w.Status)), Project: sanitize.Line(w.Project),
		Priority: sanitize.Line(w.Priority), Tags: lines(w.Tags), Urgency: w.Urgency,
		Annotations: annotations, Depends: lines(depends),
		IssueKey: sanitize.Line(w.IssueKey), IssueURL: sanitize.Line(w.IssueURL),
	}

	return task, w.readDates(&task)
}

// readDates parses each of w's dates into task.
func (w wireTask) readDates(task *Task) error {
	dates := []struct {
		into *time.Time
		text string
	}{
		{&task.Due, w.Due},
		{&task.Wait, w.Wait},
		{&task.Scheduled, w.Scheduled},
		{&task.Until, w.Until},
		{&task.Entry, w.Entry},
		{&task.Modified, w.Modified},
		{&task.Start, w.Start},
		{&task.End, w.End},
	}

	for _, date := range dates {
		parsed, err := parseDate(date.text)
		if err != nil {
			return err
		}

		*date.into = parsed
	}

	return nil
}

// decodeAnnotations reads a task's annotations.
func decodeAnnotations(wire []wireAnnotation) ([]Annotation, error) {
	if wire == nil {
		return nil, nil
	}

	annotations := make([]Annotation, 0, len(wire))

	for _, one := range wire {
		entry, err := parseDate(one.Entry)
		if err != nil {
			return nil, err
		}

		annotations = append(annotations, Annotation{Entry: entry, Description: sanitize.Line(one.Description)})
	}

	return annotations, nil
}

// decodeDepends reads depends as the array export writes, or as the
// comma-joined string older data imported into Taskwarrior may still hold.
func decodeDepends(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var uuids []string

	arrayErr := json.Unmarshal(raw, &uuids)
	if arrayErr == nil {
		return uuids, nil
	}

	var joined string

	if json.Unmarshal(raw, &joined) != nil {
		return nil, fmt.Errorf("%w: depends: %w", ErrBadOutput, arrayErr)
	}

	return strings.FieldsFunc(joined, func(character rune) bool { return character == ',' }), nil
}

// parseDate reads a date as export writes it; an absent date is the zero time.
func parseDate(text string) (time.Time, error) {
	if text == "" {
		return time.Time{}, nil
	}

	date, err := time.Parse(dateLayout, text)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %w", ErrBadOutput, err)
	}

	return date, nil
}

// lines is each of texts made safe to show on one line.
func lines(texts []string) []string {
	if texts == nil {
		return nil
	}

	safe := make([]string, 0, len(texts))

	for _, text := range texts {
		safe = append(safe, sanitize.Line(text))
	}

	return safe
}
