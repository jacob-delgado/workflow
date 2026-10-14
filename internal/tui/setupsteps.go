// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// setupStage is where the first run's form is: a Huh form asking its
// questions, the check with Jira, or the last look before the write.
type setupStage int

const (
	// stageJira asks where the file goes, Jira's address and its token.
	stageJira setupStage = iota
	stageChecking
	// stageFailed asks what to do about a check that did not pass.
	stageFailed
	// stageRest asks where the token is kept and for a webhook.
	stageRest
	stageWrite
)

// Where Jira's form opens when it is asked again: on the address, its second
// group, or on the token, its third.
const (
	atAddress = iota + 1
	atToken
)

// failedChoice is a choice offered after a check that did not pass.
type failedChoice int

// The choices offered after a check that did not pass.
const (
	failedTypeAgain failedChoice = iota
	failedTypeAddress
	failedKeep
	failedLeaveOut
)

// label is the choice's row.
func (c failedChoice) label() string {
	return map[failedChoice]string{
		failedTypeAgain:   "Type the token again",
		failedTypeAddress: "Type the address again",
		failedKeep:        "Keep them anyway",
		failedLeaveOut:    "Leave Jira out",
	}[c]
}

// setupAnswers is what the first run's forms are answered with, each bound
// to its Huh field. Every copy of the form shares it, as they share the Huh
// form that fills it in.
type setupAnswers struct {
	place    int
	address  string
	token    string
	failed   failedChoice
	keychain bool
	webhook  string
}

// answers are the answers as the file takes them, without the spaces around
// what was typed.
func (a *setupAnswers) answers() setup.Answers {
	jira := config.Jira{BaseURL: strings.TrimSpace(a.address), Token: config.Secret(strings.TrimSpace(a.token))}

	return setup.Answers{Jira: jira, Webhook: config.Secret(strings.TrimSpace(a.webhook))}
}

// jiraLeftOut reports Jira's address left blank, which leaves Jira out.
func (a *setupAnswers) jiraLeftOut() bool {
	return strings.TrimSpace(a.address) == ""
}

// leaveJiraOut forgets Jira's address and token.
func (a *setupAnswers) leaveJiraOut() {
	a.address, a.token = "", ""
}

// jiraGroups ask where the file goes, then Jira's address, then its token,
// unless the address is left blank.
func (f setupForm) jiraGroups(kit renderKit) []*huh.Group {
	place := huh.NewSelect[int]().Title("Where should the file go?").
		Description("A repository's file applies across it; your home directory's, everywhere.").
		Options(f.placeOptions(kit)...).Value(&f.bound.place)
	address := newQuietInput("Jira's address, such as https://jira.example.com", "", &f.bound.address).
		hinted("Blank leaves Jira out; the forge's issues are the tracker then.")
	token := newQuietInput("Your Jira personal access token", "", &f.bound.token).secret().
		hinted("Masked as it is typed. It is checked with Jira before it is kept.")

	return []*huh.Group{huh.NewGroup(place), huh.NewGroup(address), huh.NewGroup(token).WithHideFunc(f.bound.jiraLeftOut)}
}

// placeOptions are the places the file may go, each with what sees it.
func (f setupForm) placeOptions(kit renderKit) []huh.Option[int] {
	width := 0
	for _, shown := range f.shown {
		width = max(width, len(shown))
	}

	options := make([]huh.Option[int], 0, len(f.offer.Places))
	for index, place := range f.offer.Places {
		row := fmt.Sprintf("%-*s", width, f.shown[index]) + kit.styles.label.Render("  "+placeSeenBy(place.Place))
		options = append(options, huh.NewOption(row, index))
	}

	return options
}

// placeSeenBy is what sees a file put in place.
func placeSeenBy(place setup.Place) string {
	if place == setup.Home {
		return "your home directory"
	}

	return "this repository"
}

// failedGroups ask what to do about a check that did not pass: an address
// that is no address is typed again or left out, never kept.
func (f setupForm) failedGroups() []*huh.Group {
	choices := []failedChoice{failedTypeAgain, failedKeep, failedLeaveOut}
	if !setup.Keepable(f.checkErr) {
		choices = []failedChoice{failedTypeAddress, failedLeaveOut}
	}

	options := make([]huh.Option[failedChoice], 0, len(choices))
	for _, choice := range choices {
		options = append(options, huh.NewOption(choice.label(), choice))
	}

	f.bound.failed = choices[0]
	chosen := huh.NewSelect[failedChoice]().Title("The Jira check did not pass").Options(options...).
		Value(&f.bound.failed)

	return []*huh.Group{huh.NewGroup(chosen)}
}

// restGroups ask where the token is kept, where a keychain can keep it, and
// for a webhook.
func (f setupForm) restGroups() []*huh.Group {
	webhook := newQuietInput("A Slack incoming webhook URL", "", &f.bound.webhook).secret().
		hinted("Masked as it is typed, and saved unchecked: a webhook cannot be checked without " +
			"posting. Blank posts with your Slack user token; run `workflow slack login` for it.")

	if !f.keychainAsked() {
		return []*huh.Group{huh.NewGroup(webhook)}
	}

	keychain := huh.NewSelect[bool]().Title("Where should the token be kept?").Value(&f.bound.keychain).
		Options(huh.NewOption("In your keychain, out of the file", true),
			huh.NewOption("In the file, which only you can read", false))

	return []*huh.Group{huh.NewGroup(keychain), huh.NewGroup(webhook)}
}

// keychainAsked reports Jira kept with a keychain that can keep its token
// for the file chosen.
func (f setupForm) keychainAsked() bool {
	return !f.bound.jiraLeftOut() && f.keychainOffered()
}

// keychainOffered reports a keychain that can keep the token for the file
// chosen, in the item for Jira's address, which any file may read.
func (f setupForm) keychainOffered() bool {
	return f.offer.Places[f.bound.place].Keychain
}

// asking is the form asking groups, at stage, opened on the group at, and
// the command that starts it: drawn in the dialog or, with accessible forms
// on, handed to the prompts.
func (f setupForm) asking(m Model, stage setupStage, at int, groups ...*huh.Group) (Model, tea.Cmd) {
	form, start := newHuhForm(m.kit(), huhKeys(m.keys, "apply"), m.dialogColumns(setupWidth), m.detailRows(), groups...)
	f.stage, f.form, f.first, f.send = stage, form, form.GetFocusedField(), sendState{}

	for range at {
		start = tea.Batch(start, form.NextGroup())
	}

	m.overlay = f

	return m, m.startForm(form, start, func(err error) tea.Msg { return setupAnswered{form: form, err: err} })
}

// setupAnswered is a form of the first run's answered, or, asked as prompts,
// the error that stopped them.
type setupAnswered struct {
	form *huh.Form
	err  error
}

var _ applier = setupAnswered{}

// apply moves the first run on from the form answered, or says why its
// prompts stopped, above the form left to be answered in the dialog. A form
// since closed or replaced moves nothing.
func (msg setupAnswered) apply(m Model) (Model, tea.Cmd) {
	form, open := m.overlay.(setupForm)
	if !open || form.form != msg.form {
		return m, nil
	}

	if msg.err != nil {
		answerInPlace(msg.form, setupAnswered{form: msg.form})
		m.overlay = form.failed(msg.err)

		return m, nil
	}

	return form.answered(m)
}

// answered moves on from the form just answered: Jira's to the check, unless
// Jira was left out; the failed check's to what was chosen; and the rest's,
// the only other form, to the last look.
func (f setupForm) answered(m Model) (Model, tea.Cmd) {
	if f.stage == stageJira {
		f.who, f.bound.keychain = "", f.keychainOffered()

		if f.bound.jiraLeftOut() {
			return f.asking(m, stageRest, 0, f.restGroups()...)
		}

		return f.check(m)
	}

	if f.stage == stageFailed {
		return f.afterFailedCheck(m)
	}

	f.stage = stageWrite
	m.overlay = f

	return m, nil
}

// afterFailedCheck types the token, from blank, or the address again, keeps
// Jira's answers unchecked, or leaves Jira out, as chosen.
func (f setupForm) afterFailedCheck(m Model) (Model, tea.Cmd) {
	if f.bound.failed == failedTypeAgain {
		f.bound.token = ""

		return f.asking(m, stageJira, atToken, f.jiraGroups(m.kit())...)
	}

	if f.bound.failed == failedTypeAddress {
		return f.asking(m, stageJira, atAddress, f.jiraGroups(m.kit())...)
	}

	if f.bound.failed == failedLeaveOut {
		f.bound.leaveJiraOut()
	}

	return f.asking(m, stageRest, 0, f.restGroups()...)
}

// back goes to the question before: a group back within a form, or to the
// form before it; and from the first question, it cancels the first run.
func (f setupForm) back(m Model) (Model, tea.Cmd) {
	switch {
	case f.stage == stageWrite:
		groups := f.restGroups()

		return f.asking(m, stageRest, len(groups)-1, groups...)
	case f.form.GetFocusedField() != f.first:
		return m, f.form.PrevGroup()
	case f.stage == stageJira:
		return m.closeOverlay(), nil
	case f.stage == stageRest && f.bound.jiraLeftOut():
		return f.asking(m, stageJira, atAddress, f.jiraGroups(m.kit())...)
	}

	return f.asking(m, stageJira, atToken, f.jiraGroups(m.kit())...)
}
