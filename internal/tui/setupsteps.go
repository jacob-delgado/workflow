// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"fmt"

	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/setup"
)

// setupStep is a question the first run asks: config init's questions.
type setupStep int

const (
	stepPlace setupStep = iota
	stepJiraURL
	stepJiraToken
	stepCheckFailed
	stepKeychain
	stepWebhook
)

// setupKind is which of the first run's forms is asking: Jira's, with the
// place the file goes; the token or the address again; what to do about a
// check that did not pass; the keychain and the webhook; the webhook alone;
// or the last look before the write, which is no form.
type setupKind int

const (
	askingJira setupKind = iota
	askingToken
	askingAddress
	askingAfterFailure
	askingRest
	askingWebhook
	askingWrite
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

// label is the choice's row. A map, not a switch, so there is no last case
// left empty to fall through to the label after it.
func (c failedChoice) label() string {
	return map[failedChoice]string{
		failedTypeAgain:   "Type the token again",
		failedTypeAddress: "Type the address again",
		failedKeep:        "Keep them anyway",
		failedLeaveOut:    "Leave Jira out",
	}[c]
}

// setupQuestion is a question's words, and the hint under its field.
type setupQuestion struct {
	ask, hint string
}

// setupQuestions are each step's words. Built by a function, not held in a
// package variable, which gochecknoglobals forbids.
func setupQuestions() map[setupStep]setupQuestion {
	return map[setupStep]setupQuestion{
		stepPlace: {
			ask:  "Where should the file go?",
			hint: "A repository's file applies across it; your home directory's, everywhere.",
		},
		stepJiraURL: {
			ask:  "Jira's address, such as https://jira.example.com",
			hint: "Blank leaves Jira out; the forge's issues are the tracker then.",
		},
		stepJiraToken: {
			ask:  "Your Jira personal access token",
			hint: "Typed without showing it. It is checked with Jira before it is kept.",
		},
		stepCheckFailed: {ask: "The Jira check did not pass", hint: ""},
		stepKeychain:    {ask: "Where should the token be kept?", hint: ""},
		stepWebhook: {
			ask: "A Slack incoming webhook URL",
			hint: "Typed without showing it, and saved unchecked: a webhook cannot be checked without " +
				"posting. Blank posts with your Slack user token; run `workflow slack login` for it.",
		},
	}
}

// setupBound is what the first run's fields write into as they are answered.
// Each form binds the same one, so an answer typed in one form is there when
// another asks it again.
type setupBound struct {
	place                   int
	address, token, webhook string
	failed                  failedChoice
	keychain                bool
}

// setupGroups are the groups a form of the kind asks, and the first field it
// shows: a group whose question does not apply now is hidden. The last look
// before the write is no form, and has none.
//
//nolint:ireturn // a form's first field is any of Huh's field kinds
func (f setupForm) setupGroups(kind setupKind) ([]*huh.Group, huh.Field) {
	place, address, token := f.placeField(), f.addressField(), f.tokenField()
	keychain, webhook, failed := f.keychainField(), f.webhookField(), f.failedField()
	noToken := func() bool { return trimmed(f.bound.address) == "" }
	noKeychain := func() bool { return !f.keychainOffered() || trimmed(f.bound.address) == "" }

	groups := map[setupKind][]*huh.Group{
		askingJira:         {huh.NewGroup(place), huh.NewGroup(address), huh.NewGroup(token).WithHideFunc(noToken)},
		askingToken:        {huh.NewGroup(token)},
		askingAddress:      {huh.NewGroup(address), huh.NewGroup(token).WithHideFunc(noToken)},
		askingAfterFailure: {huh.NewGroup(failed)},
		askingRest:         {huh.NewGroup(keychain).WithHideFunc(noKeychain), huh.NewGroup(webhook)},
		askingWebhook:      {huh.NewGroup(webhook)},
		askingWrite:        nil,
	}
	firsts := map[setupKind]huh.Field{
		askingJira: place, askingToken: token, askingAddress: address, askingAfterFailure: failed,
		askingRest: keychain, askingWebhook: webhook, askingWrite: nil,
	}

	if kind == askingRest && noKeychain() {
		return groups[kind], webhook
	}

	return groups[kind], firsts[kind]
}

// placeField asks where the file goes, each place with what sees it.
func (f setupForm) placeField() *huh.Select[int] {
	width := 0
	for _, shown := range f.shown {
		width = max(width, len(shown))
	}

	options := make([]huh.Option[int], 0, len(f.offer.Places))
	for index, place := range f.offer.Places {
		options = append(options, huh.NewOption(fmt.Sprintf("%-*s  %s", width, f.shown[index], placeSeenBy(place.Place)),
			index))
	}

	question := setupQuestions()[stepPlace]

	return huh.NewSelect[int]().Title(question.ask).Description(question.hint).Options(options...).
		Value(&f.bound.place)
}

// addressField asks Jira's address.
func (f setupForm) addressField() *quietInput {
	question := setupQuestions()[stepJiraURL]

	return newQuietInput(question.ask, question.hint, "", &f.bound.address)
}

// tokenField asks Jira's token, showing none of it.
func (f setupForm) tokenField() *quietInput {
	question := setupQuestions()[stepJiraToken]

	return newQuietInput(question.ask, question.hint, "", &f.bound.token).secret()
}

// failedField asks what to do about a check that did not pass.
func (f setupForm) failedField() *huh.Select[failedChoice] {
	choices := f.failedChoices()

	options := make([]huh.Option[failedChoice], 0, len(choices))
	for _, choice := range choices {
		options = append(options, huh.NewOption(choice.label(), choice))
	}

	return huh.NewSelect[failedChoice]().Title(setupQuestions()[stepCheckFailed].ask).Options(options...).
		Value(&f.bound.failed)
}

// keychainField asks where the token is kept.
func (f setupForm) keychainField() *huh.Select[bool] {
	return huh.NewSelect[bool]().Title(setupQuestions()[stepKeychain].ask).Options(
		huh.NewOption("In your keychain, out of the file", true),
		huh.NewOption("In the file, which only you can read", false),
	).Value(&f.bound.keychain)
}

// webhookField asks the messaging webhook, showing none of it.
func (f setupForm) webhookField() *quietInput {
	question := setupQuestions()[stepWebhook]

	return newQuietInput(question.ask, question.hint, "", &f.bound.webhook).secret()
}

// placeSeenBy is what sees a file put in place.
func placeSeenBy(place setup.Place) string {
	if place == setup.Home {
		return "your home directory"
	}

	return "this repository"
}

// failedChoices are what a check that did not pass offers, in order: an
// address that is no address is typed again or left out, never kept.
func (f setupForm) failedChoices() []failedChoice {
	if !setup.Keepable(f.checkErr) {
		return []failedChoice{failedTypeAddress, failedLeaveOut}
	}

	return []failedChoice{failedTypeAgain, failedKeep, failedLeaveOut}
}

// keychainOffered reports a keychain that can keep the token for the file
// chosen, in the item for Jira's address, which any file may read.
func (f setupForm) keychainOffered() bool {
	return f.offer.Places[f.bound.place].Keychain
}

// backTo is the form esc goes back to from the first field of one of the
// kind, other than Jira's own, which esc cancels: the webhook from the last
// look; Jira's form from the address typed again; the address from the token
// typed again, and wherever Jira was left out; and the token otherwise, from
// the keychain or the check's choices.
func (f setupForm) backTo(kind setupKind) setupKind {
	switch {
	case kind == askingWrite:
		return askingWebhook
	case kind == askingAddress:
		return askingJira
	case kind == askingToken || trimmed(f.bound.address) == "":
		return askingAddress
	default:
		return askingToken
	}
}
