// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"charm.land/bubbles/v2/textinput"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// setupStep is a question the first run's form asks, in the order it asks
// them: config init's questions, one at a time.
type setupStep int

const (
	stepPlace setupStep = iota
	stepJiraURL
	stepJiraToken
	stepCheckFailed
	stepKeychain
	stepWebhook
	stepWrite
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
	switch c {
	case failedTypeAgain:
		return "Type the token again"
	case failedTypeAddress:
		return "Type the address again"
	case failedKeep:
		return "Keep them anyway"
	case failedLeaveOut:
	}

	return "Leave Jira out"
}

// The keychain's choices, in order.
const (
	keychainChosen = iota
	keychainDeclined
)

// question is what the form asks now.
func (f setupForm) question() string {
	if f.step == stepWrite {
		return "Write " + f.shown[f.place] + "?"
	}

	return setupQuestions()[f.step].ask
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
		stepWrite: {ask: "", hint: ""},
	}
}

// failedChoices are what a check that did not pass offers, in order: an
// address that is no address is typed again or left out, never kept.
func (f setupForm) failedChoices() []failedChoice {
	if !setup.Keepable(f.checkErr) {
		return []failedChoice{failedTypeAddress, failedLeaveOut}
	}

	return []failedChoice{failedTypeAgain, failedKeep, failedLeaveOut}
}

// previous is the question asked before this one.
func (f setupForm) previous() setupStep {
	switch f.step {
	case stepPlace, stepJiraURL:
		return stepPlace
	case stepJiraToken:
		return stepJiraURL
	case stepCheckFailed, stepKeychain:
		return stepJiraToken
	case stepWebhook:
		return f.beforeWebhook()
	case stepWrite:
		return stepWebhook
	}

	return stepPlace
}

// beforeWebhook is the question asked before the webhook's: the keychain's,
// Jira's token, or Jira's address when it was left out.
func (f setupForm) beforeWebhook() setupStep {
	switch {
	case f.answers.Jira.BaseURL == "":
		return stepJiraURL
	case f.keychainOffered():
		return stepKeychain
	default:
		return stepJiraToken
	}
}

// afterJiraURL is the question asked once Jira's address is answered: its
// token, or, with Jira left out, the webhook.
func (f setupForm) afterJiraURL() setupStep {
	if f.answers.Jira.BaseURL == "" {
		return stepWebhook
	}

	return stepJiraToken
}

// afterJira is the question asked once Jira's address and token are kept.
func (f setupForm) afterJira() setupStep {
	if f.keychainOffered() {
		return stepKeychain
	}

	return stepWebhook
}

// keychainOffered reports a keychain that can keep the token for the file
// chosen, in the item for Jira's address, which any file may read.
func (f setupForm) keychainOffered() bool {
	return f.offer.Places[f.place].Keychain
}

// at is the form asking step: a field to type into, empty for a credential
// and echoing nothing, or a list with the cursor on the answer given.
func (f setupForm) at(step setupStep) setupForm {
	f.step, f.choice, f.send = step, 0, sendState{}

	switch step {
	case stepPlace:
		f.choice = f.place
	case stepJiraURL:
		f.input = newInput(f.answers.Jira.BaseURL)
	case stepJiraToken, stepWebhook:
		f.input = newInput("")
		f.input.EchoMode = textinput.EchoNone
	case stepKeychain:
		if !f.keychain {
			f.choice = keychainDeclined
		}
	case stepCheckFailed, stepWrite:
	}

	return f
}

// choiceCount is how many rows the list asked now has.
func (f setupForm) choiceCount() int {
	const keychainChoices = 2

	switch f.step {
	case stepCheckFailed:
		return len(f.failedChoices())
	case stepKeychain:
		return keychainChoices
	case stepPlace, stepJiraURL, stepJiraToken, stepWebhook, stepWrite:
	}

	return len(f.offer.Places)
}

// chosen is the form with the row the cursor is on as the answer, on the
// question that answer leads to.
func (f setupForm) chosen() setupForm {
	switch f.step {
	case stepPlace:
		// The keychain is offered first wherever it can keep the token: it
		// keeps the token out of the file.
		f.place = f.choice
		f.keychain = f.keychainOffered()

		return f.at(stepJiraURL)
	case stepKeychain:
		f.keychain = f.choice == keychainChosen

		return f.at(stepWebhook)
	case stepCheckFailed:
		return f.afterFailedCheck()
	case stepJiraURL, stepJiraToken, stepWebhook, stepWrite:
	}

	return f
}

// afterFailedCheck types the token or the address again, keeps Jira's
// answers unchecked, or leaves Jira out, as chosen.
func (f setupForm) afterFailedCheck() setupForm {
	switch f.failedChoices()[f.choice] {
	case failedTypeAgain:
		return f.at(stepJiraToken)
	case failedTypeAddress:
		return f.at(stepJiraURL)
	case failedKeep:
		return f.at(f.afterJira())
	case failedLeaveOut:
	}

	f.answers.Jira = config.Jira{}

	return f.at(stepWebhook)
}
