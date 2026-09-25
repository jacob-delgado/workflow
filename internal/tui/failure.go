// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// sendState is an outbound request: whether it is in flight, and the error it
// came back with. Every overlay that posts, applies or writes carries one, and
// so does the Messaging pane's own post, so "in flight, then failed with this"
// is written and named one way rather than as a sending bool beside an err, an
// applyErr or a problem.
type sendState struct {
	sending bool
	err     error
}

// starting marks a request as in flight, clearing any earlier error.
func starting() sendState {
	return sendState{sending: true}
}

// failed records that the request came back with err, and is no longer in
// flight.
func (s sendState) failed(err error) sendState {
	return sendState{sending: false, err: err}
}

// errNeedsWriteScope leads a write the forge refused — opening, editing or
// merging a pull request, a re-run of CI, closing an issue it tracks — with its
// likeliest fix, a wider token scope, claiming no more than the forge's own
// refusal does. It is wrapped where the write is made rather than read into
// forge.ErrRefused everywhere, because a refused read wants a permission to
// read, not the write scope.
var errNeedsWriteScope = errors.New("the token may lack the write scope this needs")

// writeRefusal is a failed forge write as the interface tells it: the refusal a
// read-only token hits leads with the write scope it may lack, and any other
// failure keeps the forge's own words rather than a paraphrase of them.
func writeRefusal(err error) error {
	if errors.Is(err, forge.ErrRefused) || errors.Is(err, forge.ErrUnauthorized) {
		return fmt.Errorf("%w: %w", errNeedsWriteScope, err)
	}

	return err
}

// wording is how the interface tells an error it recognizes: briefly, for a
// summary row as narrow as a rail, and in full — with the way out — wherever
// the failure has room of its own. Every failure on screen is told through it,
// by failureBlock, failureLine, failureSummary or noticedFailure below: the
// one way the interface says something broke.
//
// An empty brief keeps the error's own words on a summary row, where they name
// the thing that failed; an empty wording keeps them everywhere, for an error
// that explains itself better than any paraphrase could.
//
// The full form names no key to press: the same sentence is told in a pane,
// where refresh retries, and in an overlay, which owns the keyboard and
// retries on enter — each surface's footer offers its own.
type wording struct {
	brief string
	full  string
}

// knownError is one sentinel the seams can return, and how it is told.
type knownError struct {
	sentinel error
	wording  wording
}

// ownWords marks a sentinel whose error already says what went wrong and what
// to do — Jira's or the forge's own reason for turning a request down, the
// messaging service's explained refusal, a program's failure status — so the
// interface shows it as it is.
func ownWords() wording {
	return wording{brief: "", full: ""}
}

// errorSentence rewrites a recognized error in the interface's voice, reporting
// false for one it does not recognize or one that keeps its own words. The
// first entry the error answers to wins, so a wrap of this package's own comes
// before the sentinel it wraps. It is built here rather than as a package map
// because gochecknoglobals forbids the latter.
func errorSentence(err error) (wording, bool) {
	for _, known := range knownErrors() {
		if errors.Is(err, known.sentinel) {
			return known.wording, known.wording.full != ""
		}
	}

	return ownWords(), false
}

// knownErrors is every sentinel a seam can return, each with how it is told.
func knownErrors() []knownError {
	return slices.Concat(localErrors(), jiraErrors(), forgeErrors(), messagingErrors(), programErrors())
}

// localErrors are this package's own wraps of a seam's error, and the refusals
// a seam's guard returns.
func localErrors() []knownError {
	return []knownError{
		{errNeedsWriteScope, wording{
			brief: "the token may lack write scope",
			full:  "The forge refused the write: the token may lack the write scope it needs. Widen it, then try again.",
		}},
		{errDryRun, wording{
			brief: "held back by dry run",
			full:  "Held back: this is a dry run, so nothing was sent.",
		}},
		{loop.ErrNothingStaged, wording{
			brief: "nothing is staged",
			full:  "nothing is staged: space stages the selected file",
		}},
	}
}

// jiraErrors are the tracker's: Jira's own, and a forge-backed tracker's
// missing issue.
func jiraErrors() []knownError {
	return []knownError{
		{jira.ErrNoCredential, wording{
			brief: "Jira is not set up",
			full:  "Jira is not set up. Add `jira.token` to `.workflow.json`.",
		}},
		{jira.ErrInvalidBaseURL, wording{
			brief: "jira.base_url is not a URL",
			full:  "`jira.base_url` is not an absolute http or https address. Fix it; `workflow doctor` checks it.",
		}},
		{jira.ErrCredentialInBaseURL, wording{
			brief: "jira.base_url holds a password",
			full:  "`jira.base_url` carries a username and password. Take them out and set `jira.token` instead.",
		}},
		{jira.ErrUnauthorized, wording{
			brief: "Jira did not accept the token",
			full:  "Jira did not accept the token. Check `jira.token`; `workflow doctor --online` tests it.",
		}},
		// Before ErrForbidden and ErrNotFound: a 403 or a 404 Jira explained
		// answers to both, and Jira's reason — a permission the account lacks, a
		// user that does not exist — beats a guess at what the status means.
		{jira.ErrRejected, ownWords()},
		{jira.ErrForbidden, wording{
			brief: "Jira refused the token",
			full:  "Jira refused the token for this. Check what its account may do; `workflow doctor --online` tests it.",
		}},
		{jira.ErrNoAPI, wording{
			brief: "no Jira API at that address",
			full:  "No Jira REST API answered at `jira.base_url`. Check the address; `workflow doctor --online` tests it.",
		}},
		{jira.ErrNotFound, wording{
			brief: "no such issue",
			full:  "No such issue: it may have moved, or the token cannot see it.",
		}},
		{jira.ErrUnexpectedStatus, wording{
			brief: "Jira answered unexpectedly",
			full:  "Jira answered with a status it does not document. Try again.",
		}},
		{jira.ErrUnreachable, wording{
			brief: "Jira did not answer",
			full:  "Jira did not answer in time. Check the VPN, then try again.",
		}},
	}
}

// forgeErrors are GitHub's and GitLab's, and the transport's both share.
func forgeErrors() []knownError {
	notOnAForge := "origin is not GitHub or GitLab"

	return []knownError{
		{forge.ErrNoToken, wording{
			brief: "no forge token",
			full:  "No forge token found. Run `gh auth login`, or set `$GITHUB_TOKEN`.",
		}},
		{forge.ErrNotARemote, wording{
			brief: notOnAForge,
			full:  "origin is not a remote workflow can read. Check it with `git remote -v`.",
		}},
		{forge.ErrUnknownForge, wording{
			brief: notOnAForge,
			full:  "workflow cannot tell which forge origin is on. Set `forge.kind` and `forge.host`.",
		}},
		{forge.ErrKindNeedsHost, wording{
			brief: "forge.kind needs forge.host",
			full:  "`forge.kind` is set without `forge.host`. Add the host it describes.",
		}},
		{forge.ErrUnauthorized, wording{
			brief: "the forge token is not valid",
			full:  "The forge did not accept the token; it may have expired. `workflow doctor --online` tests it.",
		}},
		{forge.ErrRefused, wording{
			brief: "the forge refused the request",
			full:  "The forge refused the request: the token may lack a permission this needs. Check its scopes.",
		}},
		{forge.ErrNoAPI, wording{
			brief: "no forge API at that address",
			full:  "No forge API answered at that address. Check `forge.host`; `workflow doctor --online` tests it.",
		}},
		{forge.ErrNotJSON, wording{
			brief: "the answer was not JSON",
			full:  "The forge's answer was not JSON, which usually means `forge.host` names the web host, not the API.",
		}},
		{forge.ErrRejected, ownWords()},
		{forge.ErrNoRepository, wording{
			brief: "the forge cannot see the repo",
			full:  "The forge found no such repository, or the token cannot see it. Check the token can read it.",
		}},
		{forge.ErrUnexpectedStatus, wording{
			brief: "an unexpected forge answer",
			full:  "The forge answered with a status it does not document. Try again.",
		}},
		{forge.ErrUnreachable, wording{
			brief: "could not reach the forge",
			full:  "The forge did not answer in time. Check the network, then try again.",
		}},
		{forge.ErrNoUser, wording{
			brief: "the forge has no such user",
			full:  "The forge has no user by that name. Check the reviewers and assignees.",
		}},
	}
}

// messagingErrors are the messaging service's, told the same whichever service
// it is — Slack, Teams, Discord or a plain webhook.
func messagingErrors() []knownError {
	return []knownError{
		{messaging.ErrNoCredential, wording{
			brief: "messaging is not set up",
			full:  "Messaging is not set up. Add `messaging.webhook_url` to `.workflow.json`.",
		}},
		{messaging.ErrInsecureWebhook, wording{
			brief: "the webhook is not https",
			full:  "`messaging.webhook_url` is not an https address. Copy the webhook's https address into it.",
		}},
		{messaging.ErrRejected, wording{
			brief: "the announcement was refused",
			full: "The messaging service refused the announcement: check the bot is in the channel or the " +
				"webhook is current.",
		}},
		{messaging.ErrPostRefused, ownWords()},
		{messaging.ErrUnexpectedStatus, wording{
			brief: "an unexpected service answer",
			full:  "The messaging service answered with a status it does not document. Try the announcement again.",
		}},
		{messaging.ErrUnreachable, wording{
			brief: "could not reach messaging",
			full:  "The messaging service did not answer. Check the network, then try the announcement again.",
		}},
	}
}

// programErrors are the transport's, the repository's and the programs'
// workflow runs: git, lefthook, the editor.
func programErrors() []knownError {
	return []knownError{
		{httpx.ErrRedirected, wording{
			brief: "refused to follow a redirect",
			full:  "The server answered with a redirect, refused so the token never goes elsewhere. Check the address.",
		}},
		{httpx.ErrRateLimited, wording{
			brief: "rate limited",
			full:  "Rate limited. Wait a minute, then try again.",
		}},
		{gitrepo.ErrNotARepository, wording{
			brief: "not a git repository",
			full:  "This is not inside a git repository. Start workflow from a repository's work tree.",
		}},
		{gitrepo.ErrUnshowableName, wording{
			brief: "a branch name cannot be shown",
			full:  "A branch name holds characters that cannot be shown as they are. Rename it in your shell.",
		}},
		{proc.ErrNotFound, wording{
			brief: "",
			full:  "A program workflow runs is not on PATH. Install it; `workflow doctor` names what is missing.",
		}},
		{proc.ErrTimedOut, wording{
			brief: "timed out",
			full: "A program workflow runs did not answer in time and was stopped. Check the disk or network " +
				"it reads, then try again.",
		}},
		{proc.ErrExitStatus, ownWords()},
		{editor.ErrNoSuchFile, wording{
			brief: "no such file",
			full:  "There is no such file to open; it may have moved since the tool named it.",
		}},
	}
}

// ownText is an error's own words, made safe here as well as where they were
// written.
func ownText(err error) string {
	return sanitize.Text(err.Error())
}

// ownLine is an error's own words folded onto one row.
func ownLine(err error) string {
	return strings.Join(strings.Fields(ownText(err)), " ")
}

// briefly is an error in the fewest words, for a summary row: its brief
// wording, or its own words on one row.
func briefly(err error) string {
	words, known := errorSentence(err)
	if known && words.brief != "" {
		return words.brief
	}

	return ownLine(err)
}

// inFull is an error on one row with room for the whole sentence: its full
// wording, or its own words.
func inFull(err error) string {
	words, known := errorSentence(err)
	if !known {
		return ownLine(err)
	}

	return words.full
}

// noticedFailure reports a failure in the footer: the red mark and the error in
// full, its own words after the sentence, the whole row in the failure style.
func (m Model) noticedFailure(err error) Model {
	return m.noticedFailureLedBy("", err)
}

// noticedFailureLedBy is noticedFailure with what failed leading the row —
// "cannot merge: " — for a notice no open overlay names: the footer clips a
// long sentence, and the lead must survive it.
func (m Model) noticedFailureLedBy(lead string, err error) Model {
	words := []string{m.marks.failed + " " + lead + inFull(err)}
	if _, known := errorSentence(err); known {
		words = append(words, ownText(err))
	}

	m = m.noticed(strings.Join(words, "\n"))
	m.notice.failed = true

	return m
}

// noticedGuidance tells, plainly, why a key did nothing and what would. The
// refusal is not a failure, so it takes no mark and no red — red means
// something broke — but its words live where every failure's do: in
// errorSentence, or in the refusal's own text.
func (m Model) noticedGuidance(refusal error) Model {
	return m.noticed(inFull(refusal))
}

// failedGlyph is the failure mark in red, so red always means something broke —
// the free form lets a seam that carries only styles and glyphs redden it too.
func failedGlyph(sty styles, marks glyphs) string {
	return sty.failure.Render(marks.failed)
}

// failedGlyph is failedGlyph for a Model.
func (m Model) failedGlyph() string {
	return failedGlyph(m.styles, m.marks)
}

// failureSummary is a failure on a summary row — a rail, and the line a detail
// repeats from it: the red mark and the error in brief.
func (m Model) failureSummary(err error) string {
	return m.failedGlyph() + " " + briefly(err)
}

// failureLine is a failure on a row of its own with room to spare — an
// overlay's outcome, a field's problem, a run's headline: the red mark and the
// error in full, on the one row. The free form serves an overlay, which draws
// with styles and glyphs but no Model.
func failureLine(sty styles, marks glyphs, err error) string {
	return failedGlyph(sty, marks) + " " + inFull(err)
}

// failureLine is failureLine for a Model.
func (m Model) failureLine(err error) string {
	return failureLine(m.styles, m.marks, err)
}

// failureBlock is a failure given room of its own — a pane, an overlay's pinned
// outcome: recognized as a sentence like failure, wrapped to a width and styled
// per row, so every row opens and closes its own color and none runs on into
// the border beside it.
func failureBlock(sty styles, marks glyphs, err error, width int) string {
	words, known := errorSentence(err)
	if !known {
		return sty.failure.Render(wrap(marks.failed+" "+ownText(err), width))
	}

	return sty.failure.Render(wrap(marks.failed+" "+words.full, width)) + "\n" +
		sty.label.Render(wrap(ownText(err), width))
}

// failureBlock is failureBlock for a Model.
func (m Model) failureBlock(err error, width int) string {
	return failureBlock(m.styles, m.marks, err, width)
}

// pinnedOutcome is an overlay's outcome — the in-flight word or the refusal —
// drawn under the title rather than at the bottom, so a long reason is wrapped
// and seen instead of clipped below the fold. Empty when nothing has happened.
func pinnedOutcome(sty styles, marks glyphs, send sendState, doing string, width int) []string {
	switch {
	case send.sending:
		return []string{doing + marks.ellipsis, ""}
	case send.err != nil:
		return []string{failureBlock(sty, marks, send.err, width), ""}
	default:
		return nil
	}
}
