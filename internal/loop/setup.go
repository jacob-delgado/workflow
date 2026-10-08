// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"errors"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
)

// NotSetUp reports an error that says a service was never set up to ask, as
// against one that was asked and refused: Jira or the forge has no
// credential, origin names no forge workflow reads, no Taskwarrior is
// installed or it has never run, or git has no user.email. Every surface tells
// it as guidance — what to set up — rather than as a failure.
func NotSetUp(err error) bool {
	_, notSetUp := SetUpAdvice(err)

	return notSetUp
}

// SetUpAdvice is how to set up what err says was never set up, in the words
// every surface tells it in — the web's problem, the command line's note and
// the terminal's guidance — reporting false for an error NotSetUp does not
// match. The words never name a host or a path, so a surface that must not
// show one can show them; one that may shows the error's own beneath.
func SetUpAdvice(err error) (string, bool) {
	for _, missing := range setUpCauses() {
		if errors.Is(err, missing.cause) {
			return missing.advice, true
		}
	}

	return "", false
}

// setUpCause is one error that says something was never set up, and how to
// set it up.
type setUpCause struct {
	cause  error
	advice string
}

// setUpCauses are every error NotSetUp matches, each with its advice. Built
// here rather than as a package slice because gochecknoglobals forbids the
// latter.
func setUpCauses() []setUpCause {
	return []setUpCause{
		{jira.ErrNoCredential, "Jira has no token; set jira.token, or check that jira.keychain, " +
			"jira.token_command or jira.token_env gives one — workflow doctor --online tests it"},
		{forge.ErrNoToken, "no forge token was found; for GitHub set $GITHUB_TOKEN or sign in with gh, for GitLab " +
			"set $GITLAB_TOKEN, or set forge.token; workflow doctor names where it looks for this repository"},
		{forge.ErrNotARemote, "origin does not name a repository on a forge; point it at the repository"},
		{forge.ErrUnknownForge, "cannot tell which forge this repository is on; set forge.kind and forge.host"},
		{gitrepo.ErrNoIdentity, "git has no user.email to tell your commits by; set it with git config user.email"},
		{taskwarrior.ErrNotInstalled, "Taskwarrior is not installed, or no task program is on PATH. Install " +
			"Taskwarrior " + taskwarrior.MinimumVersion + " or newer, or set taskwarrior.program."},
		{taskwarrior.ErrNotTaskwarrior, "The task on PATH is another program (go-task, most likely), not " +
			"Taskwarrior. Set taskwarrior.program to Taskwarrior's path, then restart workflow; workflow doctor " +
			"names what it found."},
		{taskwarrior.ErrNotConfigured, "Taskwarrior has never been run: run it once in a terminal so it creates " +
			"its configuration; workflow doctor names the program."},
		{messaging.ErrNoCredential, "messaging has no credential; run workflow slack login to post to Slack " +
			"with your user token, or set messaging.webhook_url (Settings on the web)"},
	}
}
