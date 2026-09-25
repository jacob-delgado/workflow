// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// spoken is a failure as a seam returns it, and how the interface tells it:
// briefly on a rail, and in full — the words a pane, an overlay and a notice
// lead with. An error that keeps its own words is told the same in both.
type spoken struct {
	err   error
	brief string
	full  string
}

// ownWordsOf is a failure the interface shows as the error itself says it.
func ownWordsOf(err error) spoken {
	return spoken{err: err, brief: err.Error(), full: err.Error()}
}

// everySeamFailure is every sentinel a seam can return, each wrapped the way a
// client wraps it, with its wording.
func everySeamFailure() map[string]spoken {
	return map[string]spoken{
		"jira no credential": {
			fmt.Errorf("searching: %w", jira.ErrNoCredential), "Jira is not set up",
			"Jira is not set up. Add `jira.token` to `.workflow.json`.",
		},
		"jira invalid base URL": {
			fmt.Errorf("searching: %w", jira.ErrInvalidBaseURL), "jira.base_url is not a URL",
			"`jira.base_url` is not an absolute http or https address. Fix it; `workflow doctor` checks it.",
		},
		"jira credential in base URL": {
			fmt.Errorf("searching: %w", jira.ErrCredentialInBaseURL), "jira.base_url holds a password",
			"`jira.base_url` carries a username and password. Take them out and set `jira.token` instead.",
		},
		"jira unauthorized": {
			fmt.Errorf("searching: %w", jira.ErrUnauthorized), "Jira did not accept the token",
			"Jira did not accept the token. Check `jira.token`; `workflow doctor --online` tests it.",
		},
		"jira forbidden": {
			fmt.Errorf("searching: %w", jira.ErrForbidden), "Jira refused the token",
			"Jira refused the token for this. Check what its account may do; `workflow doctor --online` tests it.",
		},
		"jira no API": {
			fmt.Errorf("searching: %w", jira.ErrNoAPI), "no Jira API at that address",
			"No Jira REST API answered at `jira.base_url`. Check the address; `workflow doctor --online` tests it.",
		},
		"jira not found": {
			fmt.Errorf("reading PROJ-412: %w", jira.ErrNotFound), "no such issue",
			"No such issue: it may have moved, or the token cannot see it.",
		},
		"jira rejected": ownWordsOf(jira.ErrRejected),
		"jira unexpected status": {
			fmt.Errorf("%w: 418", jira.ErrUnexpectedStatus), "Jira answered unexpectedly",
			"Jira answered with a status it does not document. Try again.",
		},
		"jira unreachable": {
			fmt.Errorf("%w at https://jira.example.com: i/o timeout", jira.ErrUnreachable), "Jira did not answer",
			"Jira did not answer in time. Check the VPN, then try again.",
		},
		"forge no token": {
			fmt.Errorf("connecting: %w", forge.ErrNoToken), "no forge token",
			"No forge token found. Run `gh auth login`, or set `$GITHUB_TOKEN`.",
		},
		"forge not a remote": {
			fmt.Errorf("reading origin: %w", forge.ErrNotARemote), "origin is not GitHub or GitLab",
			"origin is not a remote workflow can read. Check it with `git remote -v`.",
		},
		"forge unknown": {
			fmt.Errorf("reading origin: %w", forge.ErrUnknownForge), "origin is not GitHub or GitLab",
			"workflow cannot tell which forge origin is on. Set `forge.kind` and `forge.host`.",
		},
		"forge kind needs host": {
			fmt.Errorf("reading origin: %w", forge.ErrKindNeedsHost), "forge.kind needs forge.host",
			"`forge.kind` is set without `forge.host`. Add the host it describes.",
		},
		"forge unauthorized": {
			fmt.Errorf("finding: %w", forge.ErrUnauthorized), "the forge token is not valid",
			"The forge did not accept the token; it may have expired. `workflow doctor --online` tests it.",
		},
		"forge refused": {
			fmt.Errorf("finding: %w", forge.ErrRefused), "the forge refused the request",
			"The forge refused the request: the token may lack a permission this needs. Check its scopes.",
		},
		"forge no API": {
			fmt.Errorf("finding: %w", forge.ErrNoAPI), "no forge API at that address",
			"No forge API answered at that address. Check `forge.host`; `workflow doctor --online` tests it.",
		},
		"forge not JSON": {
			fmt.Errorf("finding: %w", forge.ErrNotJSON), "the answer was not JSON",
			"The forge's answer was not JSON, which usually means `forge.host` names the web host, not the API.",
		},
		"forge rejected": ownWordsOf(forge.ErrRejected),
		"forge no repository": {
			fmt.Errorf("finding: %w", forge.ErrNoRepository), "the forge cannot see the repo",
			"The forge found no such repository, or the token cannot see it. Check the token can read it.",
		},
		"forge unexpected status": {
			fmt.Errorf("%w: 418", forge.ErrUnexpectedStatus), "an unexpected forge answer",
			"The forge answered with a status it does not document. Try again.",
		},
		"forge unreachable": {
			fmt.Errorf("finding: %w", fmt.Errorf("%w at https://api.github.com: i/o timeout", forge.ErrUnreachable)),
			"could not reach the forge",
			"The forge did not answer in time. Check the network, then try again.",
		},
		"forge no user": {
			fmt.Errorf("%w: ana", forge.ErrNoUser), "the forge has no such user",
			"The forge has no user by that name. Check the reviewers and assignees.",
		},
		"messaging no credential": {
			fmt.Errorf("posting: %w", messaging.ErrNoCredential), "messaging is not set up",
			"Messaging is not set up. Add `messaging.webhook_url` to `.workflow.json`.",
		},
		"messaging insecure webhook": {
			fmt.Errorf("posting: %w", messaging.ErrInsecureWebhook), "the webhook is not https",
			"`messaging.webhook_url` is not an https address. Copy the webhook's https address into it.",
		},
		"messaging rejected": {
			fmt.Errorf("%w: invalid_token", messaging.ErrRejected), "the announcement was refused",
			"The messaging service refused the announcement: check the bot is in the channel or the webhook is current.",
		},
		"messaging post refused": {
			fmt.Errorf("%w: #dev is archived", messaging.ErrPostRefused), "the message was refused",
			"the message was refused: #dev is archived",
		},
		"messaging unexpected status": {
			fmt.Errorf("%w: 418", messaging.ErrUnexpectedStatus), "an unexpected service answer",
			"The messaging service answered with a status it does not document. Try the announcement again.",
		},
		"messaging unreachable": {
			fmt.Errorf("%w: i/o timeout", messaging.ErrUnreachable), "could not reach messaging",
			"The messaging service did not answer. Check the network, then try the announcement again.",
		},
		"a refused redirect": {
			fmt.Errorf("the server redirected the request: %w", httpx.ErrRedirected), "refused to follow a redirect",
			"The server answered with a redirect, refused so the token never goes elsewhere. Check the address.",
		},
		"rate limited": {
			fmt.Errorf("finding: %w", fmt.Errorf("%w (in 30s)", httpx.ErrRateLimited)), "rate limited",
			"Rate limited. Wait a minute, then try again.",
		},
		"not a repository": {
			fmt.Errorf("finding: %w", fmt.Errorf("%w: /tmp", gitrepo.ErrNotARepository)), "not a git repository",
			"This is not inside a git repository. Start workflow from a repository's work tree.",
		},
		"an unshowable branch name": {
			fmt.Errorf("%w: fix/\u2028", gitrepo.ErrUnshowableName), "a branch name cannot be shown",
			"A branch name holds characters that cannot be shown as they are. Rename it in your shell.",
		},
		"a program not on PATH": {
			fmt.Errorf("%w: git", proc.ErrNotFound), "program not found on PATH: git",
			"A program workflow runs is not on PATH. Install it; `workflow doctor` names what is missing.",
		},
		"a program's failure status": ownWordsOf(gitExited(1)),
		"no such file to open": {
			fmt.Errorf("opening: %w", fmt.Errorf("%w: gone.go", editor.ErrNoSuchFile)), "no such file",
			"There is no such file to open; it may have moved since the tool named it.",
		},
	}
}

// failureChannel is one of the places a failure is shown: how to put the error
// there, and the keys that bring it on screen.
type failureChannel struct {
	put   func(w *world, err error)
	keys  []string
	brief bool
}

// failureChannels are the four places a failure is told: a pane's block, an
// overlay's pinned outcome, a notice, and a rail's summary row.
func failureChannels() map[string]failureChannel {
	return map[string]failureChannel{
		"pane": {put: func(w *world, err error) { w.detailErr = err }},
		"pinned overlay": {
			put:  func(w *world, err error) { w.edited, w.commentErr = shortComment, err },
			keys: []string{"c", keyEnter},
		},
		"notice": {put: func(w *world, err error) { w.stageErr = err }, keys: []string{"3", keySpace}},
		"rail": {
			put:   func(w *world, err error) { w.pullFound, w.pullErr = false, err },
			brief: true,
		},
	}
}

// requireFailureRow fails the test unless a row of the screen shows want after
// the failure mark, with the mark in the failure color.
func requireFailureRow(t *testing.T, view, want string) {
	t.Helper()

	for row := range strings.SplitSeq(view, "\n") {
		if strings.Contains(ansi.Strip(row), failGlyph+" "+want) && strings.Contains(row, redOpen()+failGlyph) {
			return
		}
	}

	t.Errorf("no row shows %q after a red %q:\n%s", want, failGlyph, ansi.Strip(view))
}

//nolint:paralleltest // forceANSI owns the global color profile; must run serially.
func TestEveryChannelSpeaksTheFailureSentence(t *testing.T) {
	defer forceANSI(t)()

	for name, failure := range everySeamFailure() {
		for channelName, channel := range failureChannels() {
			t.Run(name+" in a "+channelName, func(t *testing.T) {
				// Arrange
				faked := newWorld()
				channel.put(faked, failure.err)

				want := failure.full
				if channel.brief {
					want = failure.brief
				}

				// Act
				view := typing(t, faked.live(t, 300, 40), channel.keys...).View().Content

				// Assert
				requireFailureRow(t, view, want)
			})
		}
	}
}
