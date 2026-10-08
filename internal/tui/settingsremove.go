// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// credentialRemoval is a credential the read holds, which removing takes out
// of the file at once, after a last look: a credential by its path, or a Jira
// header, left out of the headers.
type credentialRemoval struct {
	credential config.Credential
	header     string
}

// words names what a removal takes out.
func (r credentialRemoval) words() string {
	if r.header != "" {
		return "the header " + sanitize.Line(r.header)
	}

	return map[config.Credential]string{
		config.CredentialJiraToken:    "the Jira token",
		config.CredentialForgeToken:   "the forge token",
		config.CredentialClientSecret: "the client secret",
		config.CredentialRefreshToken: "the refresh token",
		config.CredentialWebhookURL:   "the webhook URL",
	}[r.credential]
}

// storedRemoval is the removal a row stands for, when the read holds its
// credential: a set credential, or a stored header.
func (f settingsForm) storedRemoval(field setting) (credentialRemoval, bool) {
	switch {
	case field.kind == settingSecret:
		read, _ := valueAt(f.values, field.path).(string)

		return credentialRemoval{credential: config.Credential(field.path)}, read != ""
	case field.kind == settingEntry && field.list.secret:
		read := field.list.entries(valueAt(f.values, field.path))
		stored := slices.ContainsFunc(read, func(each entry) bool { return each.name == field.entry })

		return credentialRemoval{header: field.entry}, stored
	default:
		return credentialRemoval{}, false
	}
}

// removable reports that the remove key acts on a row: an entry of a
// collection the form holds — a key only once it is moved — or a credential
// the read holds.
func (f settingsForm) removable(field setting) bool {
	_, stored := f.storedRemoval(field)

	return stored || field.kind == settingEntry && f.entryState(field, f.value(field.path)) != entry{}
}

// remove answers the remove key: a credential the read holds is held for a
// last look, and any other entry is taken out of the form, to go with the
// next save.
func (f settingsForm) remove(m Model) (Model, tea.Cmd) {
	field := f.current()

	removal, stored := f.storedRemoval(field)
	switch {
	case stored:
		m.overlay = f.askToRemove(removal)
	case f.removable(field):
		m.overlay = f.removedEntry(field)
	default:
		m.overlay = f
	}

	return m, nil
}

// askToRemove holds a removal for a last look, saying it is written at once,
// without the form's other edits, and cannot be undone.
func (f settingsForm) askToRemove(removal credentialRemoval) lastLook {
	consequence := ""
	if removal.credential == config.CredentialClientSecret || removal.credential == config.CredentialRefreshToken {
		consequence = "The access token made from it goes too. "
	}

	look := lastLook{
		title: "Remove a credential", verb: "remove", doing: "removing",
		leave: escBack, back: f,
		body: "Remove " + removal.words() + " from " + f.shownPath + "?\n\n" + consequence +
			"It cannot be undone: the file keeps no copy. It is written at once, without your other edits " +
			"here, which stay for ctrl+s.",
	}
	look.proceed = func(m Model) (Model, tea.Cmd) {
		if m.dryRun {
			m.overlay = f

			return m.noticed("dry run: would remove " + removal.words() + " from " + f.shownPath), nil
		}

		cfg, err := f.readWithout(removal)
		if err != nil {
			return keepOpenWith[lastLook](m, err), nil
		}

		write := m.deps.Settings.Save

		var removed []config.Credential
		if removal.credential != "" {
			removed = []config.Credential{removal.credential}
		}

		return m, func() tea.Msg {
			saved, written, err := write(cfg, removed, f.over)

			return settingsRemoved{form: f, removal: removal, saved: saved, over: written, err: err}
		}
	}

	return look
}

// readWithout is the configuration as read, its credentials masked, with a
// header removed: what a removal writes, the form's edits left out.
func (f settingsForm) readWithout(removal credentialRemoval) (config.Config, error) {
	values := maps.Clone(f.values)

	if removal.header != "" {
		jira, _ := values["jira"].(map[string]any)
		jira = maps.Clone(jira)
		headers, _ := jira["headers"].(map[string]any)
		headers = maps.Clone(headers)
		delete(headers, removal.header)
		jira["headers"] = headers
		values["jira"] = jira
	}

	// Trade-off TRADE-13: the configuration's JSON form always encodes.
	read, err := json.Marshal(values)
	if err != nil {
		return config.Config{}, fmt.Errorf("writing the configuration: %w", err)
	}

	cfg, err := config.Parse(bytes.NewReader(read))
	if err != nil {
		return config.Config{}, fmt.Errorf("%w: %w", errSettingsInvalid, err)
	}

	return cfg, nil
}

// settingsRemoved is a credential removed, with what was written, or why it
// was not.
type settingsRemoved struct {
	form    settingsForm
	removal credentialRemoval
	saved   config.Config
	over    config.Revision
	err     error
}

var _ applier = settingsRemoved{}

// apply goes back to Settings with the refusal, or, once removed, reopens
// workflow as a save does — unless the form holds other edits, which it then
// keeps over the file as written, saying what was removed.
func (msg settingsRemoved) apply(m Model) (Model, tea.Cmd) {
	form := msg.form

	switch {
	case msg.err != nil:
		m.overlay = form.failed(savedRefusal(msg.err))

		return m, nil
	case len(form.edits) == 0:
		return m.reopenWith(reopening{path: form.path})
	}

	form.values, form.readErr = seed(msg.saved.Redacted(), nil)
	form.over, form.said = msg.over, m.marks.done+" removed "+msg.removal.words()+"; ctrl+s saves your other edits"
	m.overlay = form.laidOut()

	return m, nil
}
