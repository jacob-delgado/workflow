// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/tui/layout"
)

// setupTitle heads the first run's form.
const setupTitle = "Set up workflow"

// setupWidth is the columns the first run's dialog would like: room for a
// question's hint on a line or two, and for Jira's address.
const setupWidth = 76

// setupForm is the first run's form, a dialog of Huh forms: where the file
// goes, Jira's address and token; the check with Jira, and what to do if it
// did not pass; where the token is kept and a messaging webhook; then a last
// look, the file written, and workflow reopened with it.
type setupForm struct {
	offer setup.Offer
	shown []string
	stage setupStage
	// form is the Huh form asking now, and first its first field, from which
	// esc leaves the form rather than going back a question in it.
	form  *huh.Form
	first huh.Field
	bound *setupAnswers
	// who is whom the token authenticates as, once the check passed, and
	// checkErr is why it did not pass.
	who      string
	checkErr error
	send     sendState
}

var (
	_ failable[setupForm] = setupForm{}
	_ pasteable           = setupForm{}
	_ relaying            = setupForm{}
	_ floating            = setupForm{}
	_ waiting             = setupForm{}
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

	form := setupForm{offer: offer, shown: shown, bound: &setupAnswers{}}

	return form.asking(m, stageJira, 0, form.jiraGroups(m.kit())...)
}

// wants is the dialog as wide as setupWidth, where the screen has room, and
// as tall as what it asks now at that width.
func (f setupForm) wants(kit renderKit, most layout.Size) layout.Size {
	width := min(setupWidth, most.Width+dialogFrameColumns)
	_, body := f.view(kit, width-dialogFrameColumns, most.Height)

	return layout.Size{Width: width, Height: lipgloss.Height(body) + dialogFrameRows}
}

// view draws the answers given so far, the form asking now, and how the
// check or the write is going.
func (f setupForm) view(kit renderKit, width, _ int) (string, string) {
	lines := []string{"No " + config.FileName + " applies here. Leave a question blank to skip it.", ""}
	if answered := f.answeredRows(kit); len(answered) > 0 {
		lines = append(append(lines, answered...), "")
	}

	lines = append(append(lines, f.asked(kit)...), f.outcome(kit, width)...)

	return setupTitle, wrap(strings.Join(lines, "\n"), width)
}

// answeredRows is a row for each answer a form before this one was given.
func (f setupForm) answeredRows(kit renderKit) []string {
	if f.stage == stageJira {
		return nil
	}

	answers := f.bound.answers()
	rows := []string{
		answeredRow(kit, "File", f.shown[f.bound.place]),
		answeredRow(kit, "Jira", orSkipped(answers.Jira.BaseURL)),
	}

	if f.who != "" {
		rows = append(rows, answeredRow(kit, "", kit.marks.done+" authenticates as "+f.who))
	}

	if f.stage != stageWrite {
		return rows
	}

	if f.keychainAsked() {
		rows = append(rows, answeredRow(kit, "Token", f.tokenKept()))
	}

	return append(rows, answeredRow(kit, "Slack", orSkipped(webhookShown(answers.Webhook))))
}

// tokenKept says where the token will be kept.
func (f setupForm) tokenKept() string {
	if f.bound.keychain {
		return "in your keychain"
	}

	return "in the file"
}

// webhookShown is a webhook as the form shows it: never its address, which
// is its credential.
func webhookShown(webhook config.Secret) string {
	if webhook == "" {
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

// asked is what the dialog asks now: the last look before the write, why
// the check did not pass above what to do about it, or the form asking.
// While the check runs it asks nothing.
func (f setupForm) asked(kit renderKit) []string {
	if f.stage == stageChecking {
		return nil
	}

	if f.stage == stageWrite {
		return []string{
			kit.styles.strong.Render("Write " + f.shown[f.bound.place] + "?"),
			kit.styles.label.Render("Then workflow reopens here with it."),
		}
	}

	if f.stage == stageFailed {
		return []string{kit.failureLine(f.checkErr), "", f.form.View()}
	}

	return []string{f.form.View()}
}

// outcome is how the check or the write is going.
func (f setupForm) outcome(kit renderKit, width int) []string {
	if f.stage == stageChecking {
		return []string{"", kit.turned + " checking the token with Jira" + kit.marks.ellipsis}
	}

	if outcome := kit.pinnedOutcome(f.send, "writing", width); outcome != nil {
		return append([]string{""}, outcome...)
	}

	return nil
}

// waits reports the check with Jira under way, whose mark turns as it waits.
func (f setupForm) waits() bool {
	return f.stage == stageChecking
}

// footer offers what the form asking now answers, or the last look's write.
func (f setupForm) footer(keys keyMap) []key.Binding {
	switch {
	case f.send.sending || f.stage == stageChecking:
		return []key.Binding{keys.interrupt}
	case f.stage == stageWrite:
		return []key.Binding{relabel(keys.confirm, "write"), relabel(keys.closeOverlay, escBack)}
	}

	return append(formKeys(f.form), f.leave(keys))
}

// leave is esc: back to the question before, or cancel at the first.
func (f setupForm) leave(keys keyMap) key.Binding {
	if f.stage == stageJira && f.form.GetFocusedField() == f.first {
		return relabel(keys.closeOverlay, escCancel)
	}

	return relabel(keys.closeOverlay, escBack)
}

// failed pins a refused write, or prompts that stopped, in the form, so it
// is read before anything else.
func (f setupForm) failed(err error) setupForm {
	f.send = f.send.failed(err)

	return f
}

// asksNothing reports a dialog with no form asking: the check under way, the
// last look, or the write.
func (f setupForm) asksNothing() bool {
	return f.stage == stageChecking || f.stage == stageWrite || f.send.sending
}

// pasted types a paste into the field asked for now, as typing it would.
func (f setupForm) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if f.asksNothing() {
		return m, nil
	}

	return f.relay(m, paste)
}

// relay hands a message to the form asking now.
func (f setupForm) relay(m Model, msg tea.Msg) (Model, tea.Cmd) {
	return m, relayToForm(f.form, msg)
}

// resized fits the form asking now to the dialog at the screen's new size.
func (f setupForm) resized(m Model) (Model, tea.Cmd) {
	return m, fitForm(f.form, m.dialogColumns(setupWidth), m.detailRows())
}

// handleKey answers a key while the dialog has the keyboard: esc goes back,
// every other key is the form's, and on the last look enter writes and
// previous-field goes back to the question before, as esc does.
func (f setupForm) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case f.stage == stageChecking || f.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return f.back(m)
	case f.stage != stageWrite:
		return f.relay(m, msg)
	case key.Matches(msg, m.keys.confirm):
		return f.write(m)
	case key.Matches(msg, m.keys.prevField):
		return f.back(m)
	}

	return m, nil
}

// which names the setup form.
func (setupForm) which() overlayKind { return overlaySetupForm }

// check asks Jira who the token is, off the update loop, its mark turning as
// it waits.
func (f setupForm) check(m Model) (Model, tea.Cmd) {
	f.stage, f.checkErr = stageChecking, nil
	m.overlay = f
	ctx, check, settings := m.session, m.deps.Settings.Setup.Check, f.bound.answers().Jira

	return m, func() tea.Msg {
		who, err := check(ctx, settings)

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
	if !open || form.stage != stageChecking {
		return m, nil
	}

	if msg.err != nil {
		form.checkErr = msg.err

		return form.asking(m, stageFailed, 0, form.failedGroups()...)
	}

	form.who = msg.who

	return form.asking(m, stageRest, 0, form.restGroups()...)
}

// write writes the file, off the update loop, or under a dry run says it
// would have.
func (f setupForm) write(m Model) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would write " + f.shown[f.bound.place]), nil
	}

	f.send = starting()
	m.overlay = f
	ctx, write := m.session, m.deps.Settings.Setup.Write
	request := setup.Request{
		Place:    f.offer.Places[f.bound.place].Place,
		Answers:  f.bound.answers(),
		Keychain: f.keychainAsked() && f.bound.keychain,
	}

	return m, func() tea.Msg {
		written, err := write(ctx, request)

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
