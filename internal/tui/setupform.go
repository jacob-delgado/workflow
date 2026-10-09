// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// setupTitle heads the first run's form.
const setupTitle = "Set up workflow"

// setupForm is the first run's form: where the file goes, Jira's address and
// token, checked against Jira as the form waits, the keychain offered for the
// token, and a messaging webhook — then the file written, and workflow
// reopened with it.
type setupForm struct {
	offer setup.Offer
	shown []string
	step  setupStep
	// choice is the row the cursor is on in a step that is a list.
	choice  int
	place   int
	input   textinput.Model
	answers setup.Answers
	// who is whom the token authenticates as, once the check passed; checking
	// marks the check under way, and checkErr is why it did not pass.
	who      string
	checking bool
	checkErr error
	keychain bool
	send     sendState
}

var (
	_ failable[setupForm] = setupForm{}
	_ pasteable           = setupForm{}
)

// offersSetup reports a session with no configuration file that can set one
// up.
func (m Model) offersSetup() bool {
	return errors.Is(m.loadErr, config.ErrNotFound) && m.deps.Settings.Setup.Write != nil
}

// setupShown reports the no-file screen on view, where enter sets one up.
func (m Model) setupShown() bool {
	_, selected := m.issues.current()

	return m.offersSetup() && (m.issues.err != nil || !selected)
}

// openSetup opens the first run's form on its first question.
func (m Model) openSetup() (Model, tea.Cmd) {
	offer := m.deps.Settings.Setup.Offer()

	shown := make([]string, 0, len(offer.Places))
	for _, place := range offer.Places {
		shown = append(shown, shownDir(m.deps, place.Path))
	}

	m.overlay = setupForm{offer: offer, shown: shown, step: stepPlace}

	return m, nil
}

// view draws the answers given so far, the question asked now, and how the
// check or the write is going.
func (f setupForm) view(kit renderKit, width, _ int) (string, string) {
	answered := f.answered(kit)
	if len(answered) > 0 {
		answered = append(answered, "")
	}

	lines := slices.Concat(
		[]string{"No " + config.FileName + " applies here. Leave a question blank to skip it.", ""},
		answered,
		[]string{kit.styles.strong.Render(f.question())},
		f.asking(kit, width),
		f.outcome(kit, width),
	)

	return setupTitle, wrap(strings.Join(lines, "\n"), width)
}

// answered is a row for each question already answered.
func (f setupForm) answered(kit renderKit) []string {
	var rows []string

	if f.step > stepPlace {
		rows = append(rows, answeredRow(kit, "File", f.shown[f.place]))
	}

	if f.step > stepJiraURL {
		rows = append(rows, answeredRow(kit, "Jira", orSkipped(f.answers.Jira.BaseURL)))
	}

	if f.who != "" && f.step > stepJiraToken {
		rows = append(rows, answeredRow(kit, "", kit.marks.done+" authenticates as "+f.who))
	}

	if f.step > stepKeychain && f.answers.Jira.BaseURL != "" && f.keychainOffered() {
		rows = append(rows, answeredRow(kit, "Token", f.tokenKept()))
	}

	if f.step > stepWebhook {
		rows = append(rows, answeredRow(kit, "Slack", orSkipped(f.webhookShown())))
	}

	return rows
}

// tokenKept says where the token will be kept.
func (f setupForm) tokenKept() string {
	if f.keychain {
		return "in your keychain"
	}

	return "in the file"
}

// webhookShown is the webhook as the form shows it: never its address, which
// is its credential.
func (f setupForm) webhookShown() string {
	if f.answers.Webhook == "" {
		return ""
	}

	return "a webhook"
}

// answeredRow is one answered question: its label, faint, and the answer.
func answeredRow(kit renderKit, label, value string) string {
	return kit.styles.label.Render(fmt.Sprintf("%-7s", label)) + value
}

// orSkipped is an answer, or that it was skipped.
func orSkipped(answer string) string {
	if answer == "" {
		return "(skipped)"
	}

	return answer
}

// asking is the field or the choices for the question asked now, and its hint.
func (f setupForm) asking(kit renderKit, width int) []string {
	switch f.step {
	case stepPlace:
		return append(f.choices(kit, f.placeChoices(kit)), kit.styles.label.Render(setupQuestions()[f.step].hint))
	case stepCheckFailed:
		return append([]string{kit.failureLine(f.checkErr), ""}, f.choices(kit, f.failedRows())...)
	case stepKeychain:
		return f.choices(kit, []string{"In your keychain, out of the file", "In the file, which only you can read"})
	case stepWrite:
		return []string{kit.styles.label.Render("Then workflow reopens here with it.")}
	case stepJiraURL, stepJiraToken, stepWebhook:
		input := f.input
		input.SetWidth(max(1, width-len(input.Prompt)-1))

		return []string{input.View(), kit.styles.label.Render(setupQuestions()[f.step].hint)}
	}

	return nil
}

// failedRows are the failed check's choices as rows.
func (f setupForm) failedRows() []string {
	choices := f.failedChoices()

	rows := make([]string, 0, len(choices))
	for _, choice := range choices {
		rows = append(rows, choice.label())
	}

	return rows
}

// placeChoices are the places the file may go, each with what sees it.
func (f setupForm) placeChoices(kit renderKit) []string {
	width := 0
	for _, shown := range f.shown {
		width = max(width, len(shown))
	}

	rows := make([]string, 0, len(f.offer.Places))
	for index, place := range f.offer.Places {
		rows = append(rows, fmt.Sprintf("%-*s", width, f.shown[index])+kit.styles.label.Render("  "+placeSeenBy(place.Place)))
	}

	return rows
}

// placeSeenBy is what sees a file put in place.
func placeSeenBy(place setup.Place) string {
	if place == setup.Home {
		return "your home directory"
	}

	return "this repository"
}

// choices are rows to choose from, the cursor's marked.
func (f setupForm) choices(kit renderKit, rows []string) []string {
	lines := make([]string, 0, len(rows))
	for index, row := range rows {
		lines = append(lines, kit.marks.marker(index == f.choice)+row)
	}

	return lines
}

// outcome is how the check or the write is going.
func (f setupForm) outcome(kit renderKit, width int) []string {
	if f.checking {
		return []string{"", "checking the token with Jira" + kit.marks.ellipsis}
	}

	if outcome := kit.pinnedOutcome(f.send, "writing", width); outcome != nil {
		return append([]string{""}, outcome...)
	}

	return nil
}

// footer offers what the question asked now takes.
func (f setupForm) footer(keys keyMap) []key.Binding {
	switch {
	case f.send.sending || f.checking:
		return []key.Binding{keys.interrupt}
	case f.step == stepWrite:
		return []key.Binding{relabel(keys.confirm, "write"), relabel(keys.closeOverlay, escBack)}
	case f.isChoice():
		return []key.Binding{keys.up, keys.down, relabel(keys.confirm, "choose"), f.leave(keys)}
	}

	return []key.Binding{relabel(keys.confirm, "next"), f.leave(keys)}
}

// leave is esc: back to the question before, or cancel at the first.
func (f setupForm) leave(keys keyMap) key.Binding {
	if f.step == stepPlace {
		return relabel(keys.closeOverlay, escCancel)
	}

	return relabel(keys.closeOverlay, escBack)
}

// isChoice reports a question answered by choosing a row.
func (f setupForm) isChoice() bool {
	return f.step == stepPlace || f.step == stepCheckFailed || f.step == stepKeychain
}

// failed pins a refused write in the form, so it is read before anything
// else.
func (f setupForm) failed(err error) setupForm {
	f.send = f.send.failed(err)

	return f
}

// pasted types a paste into the field asked for now, as typing it would.
func (f setupForm) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if f.isChoice() || f.step == stepWrite || f.checking || f.send.sending {
		return m, nil
	}

	f.input, _ = f.input.Update(paste)
	m.overlay = f

	return m, nil
}

// handleKey answers a key while the form has the keyboard: esc goes back,
// a list's keys choose, enter answers, and every other key types.
func (f setupForm) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case f.send.sending || f.checking:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return f.back(m)
	case f.isChoice():
		return f.chooseKey(m, msg)
	case key.Matches(msg, m.keys.confirm):
		return f.answer(m)
	case f.step == stepWrite:
		return m, nil
	}

	f.input, _ = f.input.Update(msg)
	m.overlay = f

	return m, nil
}

// back goes to the question before, or cancels the form at the first.
func (f setupForm) back(m Model) (Model, tea.Cmd) {
	if f.step == stepPlace {
		return m.closeOverlay(), nil
	}

	m.overlay = f.at(f.previous())

	return m, nil
}

// chooseKey moves through a list, or chooses the row the cursor is on.
func (f setupForm) chooseKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.cursorKeys()...):
		f.choice = max(0, min(f.choice+m.keys.stepOf(msg), f.choiceCount()-1))
	case key.Matches(msg, m.keys.confirm):
		m.overlay = f.chosen()

		return m, nil
	}

	m.overlay = f

	return m, nil
}

// answer takes what was typed as the answer to the question asked now, or
// writes the file once every question is answered. Only a question answered
// by typing asks: handleKey hands a list's keys to chooseKey.
func (f setupForm) answer(m Model) (Model, tea.Cmd) {
	typed := strings.TrimSpace(f.input.Value())

	if f.step == stepJiraURL {
		f.answers.Jira, f.who = config.Jira{BaseURL: typed}, ""
		m.overlay = f.at(f.afterJiraURL())

		return m, nil
	}

	if f.step == stepJiraToken {
		f.answers.Jira.Token = config.Secret(typed)

		return f.check(m)
	}

	if f.step == stepWebhook {
		f.answers.Webhook = config.Secret(typed)
		m.overlay = f.at(stepWrite)

		return m, nil
	}

	if f.step == stepWrite {
		return f.write(m)
	}

	return m, nil
}

// check asks Jira who the token is, off the update loop, saying so as it
// waits.
func (f setupForm) check(m Model) (Model, tea.Cmd) {
	f.checking, f.checkErr, f.who = true, nil, ""
	m.overlay = f
	check, settings := m.deps.Settings.Setup.Check, f.answers.Jira

	return m, func() tea.Msg {
		who, err := check(settings)

		return setupChecked{who: who, err: err}
	}
}

// setupChecked is whom the token authenticates as, or why Jira did not say.
type setupChecked struct {
	who string
	err error
}

var _ applier = setupChecked{}

// apply moves the form on past the token, or to what to do about a check
// that did not pass.
func (msg setupChecked) apply(m Model) (Model, tea.Cmd) {
	form, open := m.overlay.(setupForm)
	if !open || !form.checking {
		return m, nil
	}

	form.checking = false
	if msg.err != nil {
		form.checkErr = msg.err
		m.overlay = form.at(stepCheckFailed)

		return m, nil
	}

	form.who = msg.who
	m.overlay = form.at(form.afterJira())

	return m, nil
}

// write writes the file, off the update loop, or under a dry run says it
// would have.
func (f setupForm) write(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would write " + f.shown[f.place]), nil
	}

	f.send = starting()
	m.overlay = f
	write := m.deps.Settings.Setup.Write
	request := setup.Request{Place: f.offer.Places[f.place].Place, Answers: f.answers, Keychain: f.keychain}

	return m, func() tea.Msg {
		written, err := write(request)

		return setupWritten{written: written, err: err}
	}
}

// setupWritten is the file a first run wrote, or why it did not.
type setupWritten struct {
	written setup.Written
	err     error
}

var _ applier = setupWritten{}

// apply reopens workflow with the file written, or keeps the form open with
// why it was not.
func (msg setupWritten) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		return keepOpenWith[setupForm](m, msg.err), nil
	}

	return m.reopenWith(reopening{path: msg.written.Path, firstRun: true, notIgnored: msg.written.NotIgnored})
}
