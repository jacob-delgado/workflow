// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// setupTitle heads the first run's form.
const setupTitle = "Set up workflow"

// setupForm is the first run: where the file goes and Jira's address and
// token in one Huh form, checked against Jira as it waits, what to do about a
// check that did not pass, the keychain offered for the token and a messaging
// webhook in the next, then a last look before the file is written and
// workflow reopened with it.
type setupForm struct {
	offer setup.Offer
	shown []string
	// kind is the form asking now, form the Huh form it is and first the
	// first field it shows, where esc steps back a form rather than a field;
	// bound is what every form's fields write into.
	kind  setupKind
	form  *huh.Form
	first huh.Field
	bound *setupBound
	// answers are what was answered, as the write takes them; keychain is
	// the token kept there, who whom it authenticates as once the check
	// passed, checking the check under way, and checkErr why it did not pass.
	answers  setup.Answers
	keychain bool
	who      string
	checking bool
	checkErr error
	send     sendState
}

var (
	_ failable[setupForm] = setupForm{}
	_ pasteable           = setupForm{}
	_ relaying            = setupForm{}
	_ waiter              = setupForm{}
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

	form := setupForm{offer: offer, shown: shown, bound: &setupBound{}}

	return form.asking(m, askingJira)
}

// asking is the first run on to the form of the kind, the answers given so
// far in its fields, started; or, with accessible forms asked for, handed the
// terminal to ask it.
func (f setupForm) asking(m Model, kind setupKind) (Model, tea.Cmd) {
	f.kind, f.form, f.first, f.send = kind, nil, nil, sendState{}
	if kind == askingWrite {
		m.overlay = f

		return m, nil
	}

	groups, first := f.setupGroups(kind)
	form, start := newHuhForm(m.kit(), m.keys, m.detailWidth(), m.detailRows(), groups...)
	form.SubmitCmd = func() tea.Msg { return setupAsked{form: form} }
	f.form, f.first = form, first
	m.overlay = f

	if m.accessibleForms {
		return m, m.deps.ask(form, func(err error) tea.Msg { return setupAsked{form: form, err: err} })
	}

	return m, start
}

// setupAsked is a form of the first run answered, or, asked as prompts, the
// error that stopped them.
type setupAsked struct {
	form *huh.Form
	err  error
}

var _ applier = setupAsked{}

// apply takes the answers of the form that was asking, and goes on: from the
// check's choices, from the keychain and the webhook, or, from any other, one
// of the forms asking Jira's address and token.
func (msg setupAsked) apply(m Model) (Model, tea.Cmd) {
	form, open := m.overlay.(setupForm)
	if !open || form.form != msg.form {
		return m, nil
	}

	if msg.err != nil {
		return m.closeOverlay().noticed("set up stopped: " + inFull(msg.err)), nil
	}

	if form.kind == askingAfterFailure {
		return form.afterFailedCheck(m)
	}

	if form.kind == askingRest || form.kind == askingWebhook {
		return form.afterRest(m)
	}

	return form.afterJira(m)
}

// afterJira keeps the place and Jira's answers, then checks the token, or,
// with Jira left out, asks the rest.
func (f setupForm) afterJira(m Model) (Model, tea.Cmd) {
	if f.kind == askingJira {
		// The keychain is offered first wherever it can keep the token: it
		// keeps the token out of the file.
		f.bound.keychain = f.keychainOffered()
	}

	f.answers.Jira, f.who, f.checkErr = config.Jira{BaseURL: trimmed(f.bound.address)}, "", nil
	if f.answers.Jira.BaseURL == "" {
		return f.asking(m, askingRest)
	}

	f.answers.Jira.Token = config.Secret(trimmed(f.bound.token))

	return f.check(m)
}

// afterFailedCheck types the token or the address again, keeps Jira's
// answers unchecked, or leaves Jira out, as chosen.
func (f setupForm) afterFailedCheck(m Model) (Model, tea.Cmd) {
	switch f.bound.failed {
	case failedTypeAgain:
		f.bound.token = ""

		return f.asking(m, askingToken)
	case failedTypeAddress:
		return f.asking(m, askingAddress)
	case failedLeaveOut:
		f.answers.Jira, f.bound.address = config.Jira{}, ""
	case failedKeep:
	}

	return f.asking(m, askingRest)
}

// afterRest keeps where the token goes and the webhook, then takes a last
// look before the write.
func (f setupForm) afterRest(m Model) (Model, tea.Cmd) {
	f.keychain = f.bound.keychain && f.keychainOffered() && f.answers.Jira.BaseURL != ""
	f.answers.Webhook = config.Secret(trimmed(f.bound.webhook))

	return f.asking(m, askingWrite)
}

// trimmed is an answer without the spaces around it.
func trimmed(answer string) string {
	return strings.TrimSpace(answer)
}

// view draws the answers given so far, the form asking now, and how the
// check or the write is going.
func (f setupForm) view(kit renderKit, width, _ int) (string, string) {
	answered := f.answered(kit)
	if len(answered) > 0 {
		answered = append(answered, "")
	}

	lines := slices.Concat(
		[]string{"No " + config.FileName + " applies here. Leave a question blank to skip it.", ""},
		answered,
		f.asked(kit),
		f.outcome(kit, width),
	)

	return setupTitle, wrap(strings.Join(lines, "\n"), width)
}

// asked is the form asking now, or the last look before the write, with why
// the check did not pass above the choices it offers.
func (f setupForm) asked(kit renderKit) []string {
	switch {
	case f.kind == askingWrite:
		return []string{
			kit.styles.strong.Render("Write " + f.shown[f.bound.place] + "?"),
			kit.styles.label.Render("Then workflow reopens here with it."),
		}
	case f.checking:
		return nil
	case f.kind == askingAfterFailure:
		return []string{kit.failureLine(f.checkErr), "", f.form.View()}
	default:
		return []string{f.form.View()}
	}
}

// answered is a row for each question already answered.
func (f setupForm) answered(kit renderKit) []string {
	rows := []string{answeredRow(kit, "File", f.shown[f.bound.place])}
	if f.kind == askingJira && !f.checking {
		return nil
	}

	rows = append(rows, answeredRow(kit, "Jira", orSkipped(f.answers.Jira.BaseURL)))

	if f.who != "" {
		rows = append(rows, answeredRow(kit, "", kit.marks.done+" authenticates as "+f.who))
	}

	if f.kind == askingWrite && f.answers.Jira.BaseURL != "" && f.keychainOffered() {
		rows = append(rows, answeredRow(kit, "Token", f.tokenKept()))
	}

	if f.kind == askingWrite {
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

// outcome is how the check or the write is going.
func (f setupForm) outcome(kit renderKit, width int) []string {
	if f.checking {
		return []string{kit.marks.turning + " checking the token with Jira" + kit.marks.ellipsis}
	}

	if outcome := kit.pinnedOutcome(f.send, "writing", width); outcome != nil {
		return append([]string{""}, outcome...)
	}

	return nil
}

// waiting reports the check under way, which the in-flight mark turns for.
func (f setupForm) waiting() bool {
	return f.checking
}

// footer offers what the form asking now answers, and esc.
func (f setupForm) footer(keys keyMap) []key.Binding {
	switch {
	case f.send.sending || f.checking:
		return []key.Binding{keys.interrupt}
	case f.kind == askingWrite:
		return []key.Binding{relabel(keys.confirm, "write"), relabel(keys.closeOverlay, escBack)}
	default:
		return append(formKeys(f.form), f.leave(keys))
	}
}

// leave is esc: back a question, or cancel at the first.
func (f setupForm) leave(keys keyMap) key.Binding {
	if f.kind == askingJira && f.atFirst() {
		return relabel(keys.closeOverlay, escCancel)
	}

	return relabel(keys.closeOverlay, escBack)
}

// atFirst reports the form on the first field it shows.
func (f setupForm) atFirst() bool {
	return f.form != nil && f.form.GetFocusedField() == f.first
}

// failed pins a refused write in the form, so it is read before anything
// else.
func (f setupForm) failed(err error) setupForm {
	f.send = f.send.failed(err)

	return f
}

// pasted types a paste into the field asking now, as typing it would.
func (f setupForm) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	return f.relay(m, paste)
}

// relay hands a message to the form asking now, while one is and nothing is
// under way.
func (f setupForm) relay(m Model, msg tea.Msg) (Model, tea.Cmd) {
	if f.form == nil || f.checking || f.send.sending {
		return m, nil
	}

	return m, relayToForm(f.form, msg)
}

// handleKey answers a key while the form has the keyboard: esc goes back a
// field, or a form at a form's first, enter on the last look writes, and
// every other key is the form's.
func (f setupForm) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case f.send.sending || f.checking:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return f.back(m)
	case f.kind == askingWrite && key.Matches(msg, m.keys.confirm):
		return f.write(m)
	default:
		return f.relay(m, msg)
	}
}

// which names the setup form.
func (setupForm) which() overlayKind { return overlaySetupForm }

// back goes to the field before, the form before at a form's first, or
// cancels the first run at its first question.
func (f setupForm) back(m Model) (Model, tea.Cmd) {
	switch {
	case f.kind != askingWrite && !f.atFirst():
		return m, f.form.PrevField()
	case f.kind == askingJira:
		return m.closeOverlay(), nil
	default:
		return f.asking(m, f.backTo(f.kind))
	}
}

// check asks Jira who the token is, off the update loop, saying so as it
// waits.
func (f setupForm) check(m Model) (Model, tea.Cmd) {
	f.checking, f.checkErr, f.who = true, nil, ""
	m.overlay = f
	ctx, check, settings := m.session, m.deps.Settings.Setup.Check, f.answers.Jira

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

// apply asks the rest once the token is checked, or what to do about a check
// that did not pass.
func (msg setupChecked) apply(m Model) (Model, tea.Cmd) {
	form, open := m.overlay.(setupForm)
	if !open || !form.checking {
		return m, nil
	}

	form.checking = false
	if msg.err != nil {
		form.checkErr = msg.err
		form.bound.failed = form.failedChoices()[0]

		return form.asking(m, askingAfterFailure)
	}

	form.who = msg.who

	return form.asking(m, askingRest)
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
	request := setup.Request{Place: f.offer.Places[f.bound.place].Place, Answers: f.answers, Keychain: f.keychain}

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
