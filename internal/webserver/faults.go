// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

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

// faultClass is one kind of seam failure: the sentinels that belong to it, and
// the problem shown for it — a detail that says what to do, never the cause's
// own text, which can carry the tracker's or the forge's address.
type faultClass struct {
	causes []error
	code   api.ProblemCode
	detail string
}

// faultClasses are the failures fault tells apart, most specific first: a Jira
// 404 that carries a reason is a missing resource before it is a refusal.
func faultClasses() []faultClass {
	return slices.Concat(gitFaults(), transportFaults(), jiraFaults(), forgeFaults(), messagingFaults(),
		taskwarriorFaults(), directoryFaults(), localDataFaults(), peopleFaults())
}

// gitFaults are the repository's failures fault can name. Any other read git
// could not answer is left to the internal problem: its words can name where
// the repository is on disk, and the caller cannot act on them.
func gitFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{gitrepo.ErrNotARepository},
			code:   api.ProblemCodeConflict,
			detail: "the server is not running in a git repository; start workflow --web from a repository's work tree",
		},
		{
			causes: []error{gitrepo.ErrNoIdentity},
			code:   api.ProblemCodeUnprocessable,
			detail: setUpDetail(gitrepo.ErrNoIdentity),
		},
		{causes: []error{errBranchExists}, code: api.ProblemCodeConflict, detail: errBranchExists.Error()},
		{
			causes: []error{gitrepo.ErrIssueLinkNotSaved},
			code:   api.ProblemCodeUnprocessable,
			detail: "the link could not be kept: git could not change the repository's configuration; " +
				"check that .git/config is writable",
		},
	}
}

// transportFaults are the failures any upstream can answer with.
func transportFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{jira.ErrNotFound, forge.ErrNoRepository},
			code:   api.ProblemCodeNotFound, detail: "the requested resource was not found",
		},
		{
			causes: []error{jira.ErrUnreachable, forge.ErrUnreachable, messaging.ErrUnreachable},
			code:   api.ProblemCodeUnreachable, detail: "the service could not be reached; check the network, then try again",
		},
		// Every client answers a 429 and a redirect with the transport's own
		// sentinels, so these details name no service.
		{
			causes: []error{httpx.ErrRateLimited},
			code:   api.ProblemCodeRateLimited, detail: "the service is limiting requests; wait and try again",
		},
		{
			causes: []error{httpx.ErrRedirected},
			code:   api.ProblemCodeUnreachable,
			detail: "the service answered with a redirect, refused so the credential goes nowhere else; " +
				"check its configured address",
		},
		{
			causes: []error{httpx.ErrAnswerTooLarge},
			code:   api.ProblemCodeUnreachable, detail: "the service answered with more than workflow reads at once",
		},
		{
			causes: []error{httpx.ErrTimedOut},
			code:   api.ProblemCodeUnreachable, detail: "the service took too long to answer; try again",
		},
	}
}

// jiraFaults are the tracker's failures.
func jiraFaults() []faultClass {
	return []faultClass{
		// The client refuses the next two classes before it asks Jira anything, so
		// their details name the setting, not what Jira did.
		{
			causes: []error{jira.ErrNoCredential},
			code:   api.ProblemCodeUnprocessable,
			detail: setUpDetail(jira.ErrNoCredential),
		},
		{
			causes: []error{config.ErrInvalidBaseURL, config.ErrCredentialInBaseURL},
			code:   api.ProblemCodeUnprocessable,
			detail: "jira.base_url is not a usable address; workflow doctor checks it",
		},
		// Before the credential's class: a 403 Jira explained answers to both, and
		// doctor, which asks only who the token is, would pass a token that lacks
		// one permission.
		{
			causes: []error{jira.ErrRejected},
			code:   api.ProblemCodeUnprocessable, detail: "Jira refused the request",
		},
		{
			causes: []error{jira.ErrUnauthorized, jira.ErrForbidden},
			code:   api.ProblemCodeUnprocessable,
			detail: "Jira did not accept the configured credential; workflow doctor checks it",
		},
		{
			causes: []error{jira.ErrNoAPI, jira.ErrNotJSON},
			code:   api.ProblemCodeUnprocessable, detail: "no Jira API answers at jira.base_url; workflow doctor checks it",
		},
	}
}

// forgeFaults are GitHub's and GitLab's failures. The first two are found
// before the forge is asked anything, so their details name what to set; a
// refusal is the forge saying what the token may not do, so it points at the
// token's scopes — a rate limit is told apart before it and answered with the
// transport's own class.
func forgeFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{forge.ErrNoToken},
			code:   api.ProblemCodeUnprocessable,
			// Not the resolver's own words: for a host other than github.com or
			// gitlab.com they name the host, which a detail never does.
			detail: setUpDetail(forge.ErrNoToken),
		},
		{
			causes: []error{forge.ErrKindNeedsHost},
			code:   api.ProblemCodeUnprocessable, detail: "forge.kind is set without forge.host; set the host it describes",
		},
		{
			causes: []error{forge.ErrUnknownForge},
			code:   api.ProblemCodeUnprocessable, detail: setUpDetail(forge.ErrUnknownForge),
		},
		{
			causes: []error{forge.ErrNotARemote},
			code:   api.ProblemCodeUnprocessable, detail: setUpDetail(forge.ErrNotARemote),
		},
		{
			causes: []error{forge.ErrNoAPI, forge.ErrNotJSON},
			code:   api.ProblemCodeUnprocessable,
			detail: "no forge API answered; check forge.host, which workflow doctor --online tests",
		},
		// A job log GitHub hands to storage elsewhere: neither class is the
		// token's doing, and the storage's address is signed, so it stays out.
		{
			causes: []error{forge.ErrInsecureLog, forge.ErrLogNotRedirected},
			code:   api.ProblemCodeUnreachable,
			detail: "the forge sent the log somewhere it could not be read from safely; open the check's page instead",
		},
		{
			causes: []error{forge.ErrLogStorage},
			code:   api.ProblemCodeUnreachable,
			detail: "the storage the forge keeps the log in did not hand it over; try again, or open the check's page",
		},
		{
			// An explained status is the same undocumented status with the
			// forge's reason, which stays off the wire.
			causes: []error{forge.ErrUnexpectedStatus, forge.ErrRejected},
			code:   api.ProblemCodeUnreachable, detail: "the forge answered with a status it does not document; try again",
		},
	}
}

// messagingFaults are the messaging service's failures, told the same whichever
// service it is. None forwards the error's own text: a client's error can name
// the webhook, whose address is its credential. A message refused for its
// channel is faultProblem's, which tells its reason.
func messagingFaults() []faultClass {
	return []faultClass{
		{
			causes: []error{messaging.ErrNoCredential},
			code:   api.ProblemCodeUnprocessable,
			detail: setUpDetail(messaging.ErrNoCredential),
		},
		{
			causes: []error{messaging.ErrInsecureWebhook},
			code:   api.ProblemCodeUnprocessable,
			detail: "messaging.webhook_url is not an https address; copy the webhook's https address into it in Settings",
		},
		// Every 4xx a post meets is this one, and a webhook — the only transport
		// Teams, Discord and a plain webhook have — is one workflow doctor cannot
		// check, so the detail points at the settings rather than at doctor.
		{
			causes: []error{messaging.ErrRejected},
			code:   api.ProblemCodeUnprocessable,
			detail: "the messaging service refused the post; " +
				"check the token, or that the webhook URL is current, in Settings",
		},
		{
			causes: []error{messaging.ErrUnexpectedStatus},
			code:   api.ProblemCodeUnreachable,
			detail: "the messaging service answered with a status it does not document; try again",
		},
	}
}

// taskwarriorFaults are Taskwarrior's failures that have fixed words: a write it
// declined (a declined start ahead of the rest, since it has words of its own),
// a sync with nowhere to go, and each reason no Taskwarrior can be asked, worded
// as the task list words it. A refusal and a
// timeout are taskFault's, and an answer that is not Taskwarrior's JSON is left
// to the internal problem: nothing the caller can act on.
func taskwarriorFaults() []faultClass {
	reasons := taskwarriorReasons()

	unavailable := make([]faultClass, 0, len(reasons))
	for _, reason := range reasons {
		unavailable = append(unavailable,
			faultClass{causes: []error{reason.cause}, code: api.ProblemCodeUnprocessable, detail: reason.text})
	}

	return slices.Concat([]faultClass{
		{
			causes: []error{errStartDeclined},
			code:   api.ProblemCodeConflict, detail: "the task is already started, or no such task exists",
		},
		{
			causes: []error{taskwarrior.ErrNothingChanged},
			code:   api.ProblemCodeConflict, detail: "the task is already in that state, or is no longer pending",
		},
		{
			causes: []error{taskwarrior.ErrNoSync},
			code:   api.ProblemCodeUnprocessable,
			detail: "No sync backend is set in your taskrc, so there is nowhere to sync. " +
				"Set one of the sync.* settings (task-sync(5)).",
		},
	}, unavailable)
}

// taskwarriorReason is why no Taskwarrior can be asked, as the code the task
// list gives it and in words safe to show: never the error's own, which name
// the task program's path.
type taskwarriorReason struct {
	cause error
	code  api.TaskListReasonCode
	text  string
}

// taskwarriorReasons are the reasons the task list and a refused write share.
func taskwarriorReasons() []taskwarriorReason {
	return []taskwarriorReason{
		{
			cause: taskwarrior.ErrNotInstalled, code: api.TaskListReasonCodeNotInstalled,
			text: setUpDetail(taskwarrior.ErrNotInstalled),
		},
		{
			cause: taskwarrior.ErrNotTaskwarrior, code: api.TaskListReasonCodeNotTaskwarrior,
			text: setUpDetail(taskwarrior.ErrNotTaskwarrior),
		},
		{
			cause: taskwarrior.ErrTooOld, code: api.TaskListReasonCodeTooOld,
			text: "Taskwarrior is too old: " + taskwarrior.MinimumVersion +
				" or newer is needed; workflow doctor shows the version found.",
		},
		{
			cause: taskwarrior.ErrNotConfigured, code: api.TaskListReasonCodeNeverRun,
			text: setUpDetail(taskwarrior.ErrNotConfigured),
		},
	}
}

// directoryFaults are a directory's failures, in words that never repeat its
// path, which the error itself carries.
func directoryFaults() []faultClass {
	return []faultClass{
		{causes: []error{workdirs.ErrNotFound}, code: api.ProblemCodeNotFound, detail: "there is no such directory"},
		{
			causes: []error{workdirs.ErrNotADirectory, workdirs.ErrNotAbsolute, store.ErrNotADirectoryPath},
			code:   api.ProblemCodeUnprocessable, detail: "that is not an absolute path to a directory",
		},
		{
			causes: []error{workdirs.ErrUnreadable}, code: api.ProblemCodeUnprocessable,
			detail: "that directory cannot be read; check its permissions",
		},
		{
			causes: []error{errFavoritesNotKept, errNoSwitching}, code: api.ProblemCodeUnprocessable,
			detail: "that is not available here: the store is turned off, or the server cannot switch",
		},
		{
			causes: []error{store.ErrKeptSchemaDiffers}, code: api.ProblemCodeUnprocessable,
			detail: store.ErrKeptSchemaDiffers.Error(),
		},
		{
			causes: []error{ErrConfigurationUnreadable}, code: api.ProblemCodeUnprocessable,
			detail: "the configuration there did not load; workflow doctor there says why",
		},
		{
			causes: []error{ErrConfigurationRefused}, code: api.ProblemCodeUnprocessable,
			detail: "the configuration there binds keys workflow refuses; workflow doctor there names them",
		},
	}
}
