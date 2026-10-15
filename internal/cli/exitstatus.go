// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/cli/issues"
	"github.com/jacob-delgado/workflow/internal/cli/pr"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/slackauth"
	"github.com/jacob-delgado/workflow/internal/store"
)

// Exit statuses, so a script can tell one kind of failure from another
// without reading the message. docs/content/docs/scripting.md is their
// contract; they mirror the web's problem codes (docs/content/docs/errors.md).
const (
	exitSuccess       = 0
	exitFailure       = 1
	exitUsage         = 2
	exitConfiguration = 3
	exitRefused       = 4
	exitUnreachable   = 5
	// exitInterrupted is 128 plus SIGINT's number, what a shell reports for a
	// program stopped by Ctrl+C.
	exitInterrupted = 130
)

// ExitStatus is the status the process exits with for err: 0 for none, 130 when
// it was interrupted, 1 when its output could not be written, and otherwise the
// first family err belongs to — usage 2, configuration 3, a refused
// precondition 4, unreachable 5 — or 1 for any other failure. A joined error
// takes the first family any of its errors is in, in that order.
func ExitStatus(err error) int {
	if err == nil {
		return exitSuccess
	}

	// Checked first: an interrupted request also wraps its service's
	// unreachable error, and whoever pressed Ctrl+C is owed 130, not 5.
	if errors.Is(err, context.Canceled) {
		return exitInterrupted
	}

	// An artifact that went nowhere is the run's failure whatever else it
	// found, which a script never saw.
	if errors.Is(err, cmdkit.ErrOutputUnwritten) {
		return exitFailure
	}

	for _, family := range exitFamilies() {
		if family.holds(err) {
			return family.status
		}
	}

	return exitFailure
}

// exitFamily is an exit status and the errors that earn it.
type exitFamily struct {
	status  int
	members []error
}

// holds reports whether err is, or wraps, one of the family's errors.
func (f exitFamily) holds(err error) bool {
	return cmdkit.IsAny(err, f.members...)
}

// exitFamilies are the kinds of failure a script can tell apart, in the order a
// joined error is classified. Built by a function rather than held in a
// package-level variable, which gochecknoglobals forbids.
func exitFamilies() []exitFamily {
	return []exitFamily{
		{status: exitUsage, members: []error{
			cmdkit.ErrUsage, cmdkit.ErrNoTerminal, errLoginBlank,
			issues.ErrCommentBlank, issues.ErrStatusBlank, issues.ErrAssigneeBlank,
			issues.ErrDurationBlank, issues.ErrWorklogOnForge,
		}},
		{status: exitConfiguration, members: configurationErrors()},
		{status: exitRefused, members: refusalErrors()},
		{status: exitUnreachable, members: append(cmdkit.ServicesUnreachable(), errUnreachable)},
	}
}

// configurationErrors are a configuration that is missing, unreadable,
// incomplete — a credential or an address not set, or set unusably — shared,
// holding a credential a service would not accept, or a ui.keys map the
// interface refuses, a Slack login the messaging section cannot take or whose
// credentials Slack will not refresh, a worklog with no Jira to log it in, and
// a repository with no remote to name the forge, or one on a host neither it
// nor forge.kind names: what the user fixes in the file, the environment, a
// login or the repository's remotes. A command meeting a credential that is
// not there exits as doctor does, reading the same file.
func configurationErrors() []error {
	return append([]error{
		config.ErrNotFound, config.ErrInvalid,
		jira.ErrNoCredential, config.ErrInvalidBaseURL, config.ErrCredentialInBaseURL,
		forge.ErrNoToken, forge.ErrKindNeedsHost, forge.ErrNotARemote, forge.ErrUnknownForge,
		messaging.ErrNoCredential, messaging.ErrInsecureWebhook,
		jira.ErrUnauthorized, jira.ErrForbidden, forge.ErrUnauthorized, messaging.ErrRejected,
		errCredentialRejected, errCredentialMissing, errIncomplete, errInvalid, errShared,
		cmdkit.ErrMessagingNotConfigured, errLoginNeedsConfig, errLoginOverWebhook, errLoginNotSlack,
		issues.ErrWorklogNeedsJira,
		slackauth.ErrRefreshRefused, slackauth.ErrNotAUserToken,
	}, config.KeyRefusals()...)
}

// refusalErrors are a command that would not go ahead because of the state it
// found: an open pull request, nothing to open, a dirty tree, a branch or file
// already there, a branch that cannot be finished, a directory that is no
// repository, a store file another program holds open, a summary too long for
// the messaging service, a move to a status the tracker does not offer or one
// that asks for fields.
func refusalErrors() []error {
	return []error{
		loop.ErrPullAlreadyOpen, loop.ErrNothingToOpen, loop.ErrNoPullRequest,
		loop.ErrDirtyTree, loop.ErrNothingStaged,
		loop.ErrNotOnFeatureBranch, loop.ErrNotMerged, loop.ErrNoBase, loop.ErrUnpushedWork,
		pr.ErrPullAlreadyOpen, pr.ErrNoCommitsToOpen, pr.ErrNoPullRequest,
		errBranchExists, setup.ErrExists, gitrepo.ErrNotARepository, store.ErrNotCleaned,
		messaging.ErrTooLong, loop.ErrNoTransition, loop.ErrNeedsFields,
		loop.ErrCIFailed, loop.ErrNoCI, loop.ErrPullChanged, loop.ErrMergedFirst, pr.ErrOnlyReadyWaits,
	}
}
