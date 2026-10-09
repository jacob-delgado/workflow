// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"slices"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// Faults are the failures every surface tells alike, by the system each
// belongs to, so a surface that tells failures of its own apart can place
// them among these where they belong: within a group, the more specific
// first — a Jira 404 that carries a reason is a missing resource before it is
// a refusal.
type Faults struct {
	Git, Transport, Jira, Forge, Messaging, Taskwarrior, Directory []Class
}

// SharedFaults are the failures every surface tells alike.
func SharedFaults() Faults {
	return Faults{
		Git: gitFaults(), Transport: transportFaults(), Jira: jiraFaults(), Forge: forgeFaults(),
		Messaging: messagingFaults(), Taskwarrior: taskwarriorFaults(), Directory: directoryFaults(),
	}
}

// All is every group's classes, in the order they are told apart.
func (f Faults) All() []Class {
	return slices.Concat(f.Git, f.Transport, f.Jira, f.Forge, f.Messaging, f.Taskwarrior, f.Directory)
}

// gitFaults are the repository's failures a surface can name. Any other read git
// could not answer is left to the internal problem: its words can name where
// the repository is on disk, and the caller cannot act on them.
func gitFaults() []Class {
	return []Class{
		{
			Causes: []error{gitrepo.ErrNotARepository},
			Code:   api.ProblemCodeConflict,
			Detail: "not a git repository; start workflow from a repository's work tree",
		},
		{
			Causes: []error{gitrepo.ErrNoIdentity},
			Code:   api.ProblemCodeUnprocessable,
			Detail: setUpDetail(gitrepo.ErrNoIdentity),
		},
		{
			Causes: []error{gitrepo.ErrIssueLinkNotSaved},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "the link could not be kept: git could not change the repository's configuration; " +
				"check that .git/config is writable",
		},
	}
}

// transportFaults are the failures any upstream can answer with.
func transportFaults() []Class {
	return []Class{
		{
			Causes: []error{jira.ErrNotFound, forge.ErrNoRepository},
			Code:   api.ProblemCodeNotFound, Detail: "the requested resource was not found",
		},
		{
			Causes: []error{jira.ErrUnreachable, forge.ErrUnreachable, messaging.ErrUnreachable},
			Code:   api.ProblemCodeUnreachable, Detail: "the service could not be reached; check the network, then try again",
		},
		// Every client answers a 429 and a redirect with the transport's own
		// sentinels, so these details name no service.
		{
			Causes: []error{httpx.ErrRateLimited},
			Code:   api.ProblemCodeRateLimited, Detail: "the service is limiting requests; wait and try again",
		},
		{
			Causes: []error{httpx.ErrRedirected},
			Code:   api.ProblemCodeUnreachable,
			Detail: "the service answered with a redirect, refused so the credential goes nowhere else; " +
				"check its configured address",
		},
		{
			Causes: []error{httpx.ErrAnswerTooLarge},
			Code:   api.ProblemCodeUnreachable, Detail: "the service answered with more than workflow reads at once",
		},
		{
			Causes: []error{httpx.ErrTimedOut},
			Code:   api.ProblemCodeUnreachable, Detail: "the service took too long to answer; try again",
		},
	}
}

// jiraFaults are the tracker's failures.
func jiraFaults() []Class {
	return []Class{
		// The client refuses the next two classes before it asks Jira anything, so
		// their details name the setting, not what Jira did.
		{
			Causes: []error{jira.ErrNoCredential},
			Code:   api.ProblemCodeUnprocessable,
			Detail: setUpDetail(jira.ErrNoCredential),
		},
		{
			Causes: []error{config.ErrInvalidBaseURL, config.ErrCredentialInBaseURL},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "jira.base_url is not a usable address; workflow doctor checks it",
		},
		// Before the credential's class: a 403 Jira explained answers to both, and
		// doctor, which asks only who the token is, would pass a token that lacks
		// one permission.
		{
			Causes: []error{jira.ErrRejected},
			Code:   api.ProblemCodeUnprocessable, Detail: "Jira refused the request",
		},
		{
			Causes: []error{jira.ErrUnauthorized, jira.ErrForbidden},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "Jira did not accept the configured credential; workflow doctor checks it",
		},
		{
			Causes: []error{jira.ErrNoAPI, jira.ErrNotJSON},
			Code:   api.ProblemCodeUnprocessable, Detail: "no Jira API answers at jira.base_url; workflow doctor checks it",
		},
	}
}

// forgeFaults are GitHub's and GitLab's failures. The first two are found
// before the forge is asked anything, so their details name what to set; a
// refusal is the forge saying what the token may not do, so it points at the
// token's scopes — a rate limit is told apart before it and answered with the
// transport's own class.
func forgeFaults() []Class {
	return []Class{
		{
			Causes: []error{forge.ErrNoToken},
			Code:   api.ProblemCodeUnprocessable,
			// Not the resolver's own words: for a host other than github.com or
			// gitlab.com they name the host, which a detail never does.
			Detail: setUpDetail(forge.ErrNoToken),
		},
		{
			Causes: []error{forge.ErrKindNeedsHost},
			Code:   api.ProblemCodeUnprocessable, Detail: "forge.kind is set without forge.host; set the host it describes",
		},
		{
			Causes: []error{forge.ErrUnknownForge},
			Code:   api.ProblemCodeUnprocessable, Detail: setUpDetail(forge.ErrUnknownForge),
		},
		{
			Causes: []error{forge.ErrNotARemote},
			Code:   api.ProblemCodeUnprocessable, Detail: setUpDetail(forge.ErrNotARemote),
		},
		{
			Causes: []error{forge.ErrNoAPI, forge.ErrNotJSON},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "no forge API answered; check forge.host, which workflow doctor --online tests",
		},
		// A job log GitHub hands to storage elsewhere: neither class is the
		// token's doing, and the storage's address is signed, so it stays out.
		{
			Causes: []error{forge.ErrInsecureLog, forge.ErrLogNotRedirected},
			Code:   api.ProblemCodeUnreachable,
			Detail: "the forge sent the log somewhere it could not be read from safely; open the check's page instead",
		},
		{
			Causes: []error{forge.ErrLogStorage},
			Code:   api.ProblemCodeUnreachable,
			Detail: "the storage the forge keeps the log in did not hand it over; try again, or open the check's page",
		},
		{
			// An explained status is the same undocumented status with the
			// forge's reason, which stays off the wire.
			Causes: []error{forge.ErrUnexpectedStatus, forge.ErrRejected},
			Code:   api.ProblemCodeUnreachable, Detail: "the forge answered with a status it does not document; try again",
		},
	}
}

// messagingFaults are the messaging service's failures, told the same whichever
// service it is. None forwards the error's own text: a client's error can name
// the webhook, whose address is its credential. A message refused for its
// channel is Classify's, which tells its reason.
func messagingFaults() []Class {
	return []Class{
		{
			Causes: []error{messaging.ErrNoCredential},
			Code:   api.ProblemCodeUnprocessable,
			Detail: setUpDetail(messaging.ErrNoCredential),
		},
		{
			Causes: []error{messaging.ErrInsecureWebhook},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "messaging.webhook_url is not an https address; copy the webhook's https address into it in Settings",
		},
		// A credential a post meets refused — the token, or a webhook that no
		// longer works — is this one, and a webhook, the only transport Teams,
		// Discord and a plain webhook have, is one workflow doctor cannot check,
		// so the detail points at the settings rather than at doctor.
		{
			Causes: []error{messaging.ErrRejected},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "the messaging service refused the post; " +
				"check the token, or that the webhook URL is current, in Settings",
		},
		{
			Causes: []error{messaging.ErrUnexpectedStatus},
			Code:   api.ProblemCodeUnreachable,
			Detail: "the messaging service answered with a status it does not document; try again",
		},
	}
}

// taskwarriorFaults are Taskwarrior's failures that have fixed words: a write
// that changed nothing, a sync with nowhere to go, and each reason no
// Taskwarrior can be asked, worded as the task list words it. A refusal and a
// timeout are the web's to word, and an answer that is not Taskwarrior's JSON
// is left to the internal problem: nothing the caller can act on.
func taskwarriorFaults() []Class {
	reasons := TaskwarriorReasons()

	unavailable := make([]Class, 0, len(reasons))
	for _, reason := range reasons {
		unavailable = append(unavailable,
			Class{Causes: []error{reason.Cause}, Code: api.ProblemCodeUnprocessable, Detail: reason.Text})
	}

	return slices.Concat([]Class{
		{
			Causes: []error{taskwarrior.ErrNothingChanged},
			Code:   api.ProblemCodeConflict, Detail: "the task is already in that state, or is no longer pending",
		},
		{
			Causes: []error{taskwarrior.ErrNoSync},
			Code:   api.ProblemCodeUnprocessable,
			Detail: "No sync backend is set in your taskrc, so there is nowhere to sync. " +
				"Set one of the sync.* settings (task-sync(5)).",
		},
	}, unavailable)
}

// TaskwarriorReason is why no Taskwarrior can be asked, as the code the task
// list gives it and in words safe to show: never the error's own, which name
// the task program's path.
type TaskwarriorReason struct {
	Cause error
	Code  api.TaskListReasonCode
	Text  string
}

// TaskwarriorReasons are the reasons the task list and a refused write share.
func TaskwarriorReasons() []TaskwarriorReason {
	return []TaskwarriorReason{
		{
			Cause: taskwarrior.ErrNotInstalled, Code: api.TaskListReasonCodeNotInstalled,
			Text: setUpDetail(taskwarrior.ErrNotInstalled),
		},
		{
			Cause: taskwarrior.ErrNotTaskwarrior, Code: api.TaskListReasonCodeNotTaskwarrior,
			Text: setUpDetail(taskwarrior.ErrNotTaskwarrior),
		},
		{
			Cause: taskwarrior.ErrTooOld, Code: api.TaskListReasonCodeTooOld,
			Text: "Taskwarrior is too old: " + taskwarrior.MinimumVersion +
				" or newer is needed; workflow doctor shows the version found.",
		},
		{
			Cause: taskwarrior.ErrNotConfigured, Code: api.TaskListReasonCodeNeverRun,
			Text: setUpDetail(taskwarrior.ErrNotConfigured),
		},
	}
}

// directoryFaults are a directory's failures, in words that never repeat its
// path, which the error itself carries.
func directoryFaults() []Class {
	return []Class{
		{Causes: []error{workdirs.ErrNotFound}, Code: api.ProblemCodeNotFound, Detail: "there is no such directory"},
		{
			Causes: []error{workdirs.ErrNotADirectory, workdirs.ErrNotAbsolute, store.ErrNotADirectoryPath},
			Code:   api.ProblemCodeUnprocessable, Detail: "that is not an absolute path to a directory",
		},
		{
			Causes: []error{workdirs.ErrUnreadable}, Code: api.ProblemCodeUnprocessable,
			Detail: "that directory cannot be read; check its permissions",
		},
		{
			Causes: []error{store.ErrKeptSchemaDiffers}, Code: api.ProblemCodeUnprocessable,
			Detail: store.ErrKeptSchemaDiffers.Error(),
		},
	}
}
