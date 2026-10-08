// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/editor"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
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

// wording is how the interface tells an error it recognizes: briefly, for a
// summary row as narrow as a rail, and in full — with the way out — wherever
// the failure has room of its own. Every failure on screen is told through it,
// by failureBlock, failureLine, failureSummary, unreadRow or noticedFailure
// below: the one way the interface says something broke — and, through the
// same helpers, the one way it says something was never set up, as guidance
// rather than as a failure (see voice).
//
// An empty brief keeps the error's own words on a summary row, where they name
// the thing that failed; an empty full keeps them wherever the failure has
// room, for an error that explains itself better than any paraphrase could.
//
// The full form names no key to press: the same sentence is told in a pane,
// where refresh retries, and in an overlay, which owns the keyboard and
// retries on enter — each surface's footer offers its own.
type wording struct {
	brief string
	full  string
	// whole records that full already carries the error's own reason — the
	// forge's words for a token it turned down — so they are not told again
	// beneath it.
	whole bool
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
	return wording{brief: "", full: "", whole: false}
}

// errorSentence rewrites a recognized error in the interface's voice, reporting
// false for one it does not recognize or one that keeps its own words. The
// first entry the error answers to wins, so a wrap of this package's own comes
// before the sentinel it wraps. It is built here rather than as a package map
// because gochecknoglobals forbids the latter.
func errorSentence(err error) (wording, bool) {
	for _, known := range knownErrors() {
		if errors.Is(err, known.sentinel) {
			words := advised(known.wording, err)

			return words, words.full != ""
		}
	}

	return ownWords(), false
}

// advised is a forge's refusal of its token told in the words forge.Advice
// gives it, the same the web shows, naming the forge, the scope it asks for and
// what it said; any other error keeps words.
func advised(words wording, err error) wording {
	advice, ok := forge.Advice(err)
	if !ok {
		return words
	}

	// Only a refusal the seam returned as it is carries nothing the advice does
	// not: one wrapped in the step it failed at keeps its own words beneath, so
	// that step is still named.
	_, bare := err.(*forge.RefusalError) //nolint:errorlint // the unwrapped refusal alone says nothing more

	return wording{brief: words.brief, full: advice, whole: bare}
}

// knownErrors is every sentinel a seam can return, each with how it is told.
// Taskwarrior's come before the programs', so a sentinel of theirs wins over any
// program error it might wrap.
func knownErrors() []knownError {
	return slices.Concat(localErrors(), jiraErrors(), forgeErrors(), messagingErrors(), taskwarriorErrors(),
		programErrors())
}

// localErrors are this package's own wraps of a seam's error, and the refusals
// a seam's guard returns.
func localErrors() []knownError {
	return []knownError{
		{errDryRun, wording{
			brief: "held back by dry run",
			full:  "Held back: this is a dry run, so nothing was sent.",
		}},
		{loop.ErrNothingStaged, wording{
			brief: "nothing is staged",
			full:  "nothing is staged: stage a file, then commit",
		}},
	}
}

// setUp is the wording of a cause that says something was never set up: the
// brief a rail has room for, and in full the advice every surface shares, so
// the terminal says how to set it up in the web's and the command line's words.
func setUp(brief string, cause error) knownError {
	advice, _ := loop.SetUpAdvice(cause)

	return knownError{cause, wording{brief: brief, full: advice, whole: false}}
}

// jiraErrors are the tracker's: Jira's own, and a forge-backed tracker's
// missing issue.
func jiraErrors() []knownError {
	return []knownError{
		setUp("Jira has no token", jira.ErrNoCredential),
		{config.ErrInvalidBaseURL, wording{
			brief: "jira.base_url is not a URL",
			full:  "`jira.base_url` is not an absolute http or https address. Fix it; `workflow doctor` checks it.",
		}},
		{config.ErrCredentialInBaseURL, wording{
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
		// Not the advice the web shares: only the resolver knows the forge and
		// host, and the terminal, unlike the web, may name them, so its own words
		// say where to set this repository's token — gh for GitHub,
		// $GITLAB_TOKEN for GitLab — rather than naming both.
		{forge.ErrNoToken, wording{brief: "no forge token", full: ""}},
		setUp(notOnAForge, forge.ErrNotARemote),
		setUp(notOnAForge, forge.ErrUnknownForge),
		{forge.ErrKindNeedsHost, wording{
			brief: "forge.kind needs forge.host",
			full:  "`forge.kind` is set without `forge.host`. Add the host it describes.",
		}},
		// The full sentence for a token the forge turned down is forge.Advice's,
		// the one the web shows too, naming the forge and the scope it asks for.
		{forge.ErrUnauthorized, wording{brief: "the forge token is not valid", full: "", whole: false}},
		{forge.ErrRefused, wording{brief: "the forge refused the request", full: "", whole: false}},
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
		setUp("messaging has no credential", messaging.ErrNoCredential),
		{messaging.ErrInsecureWebhook, wording{
			brief: "the webhook is not https",
			full:  "`messaging.webhook_url` is not an https address. Copy the webhook's https address into it.",
		}},
		{messaging.ErrRejected, wording{
			brief: "the post was refused",
			full: "The messaging service refused the post: run `workflow slack login` again, " +
				"check that you are in the channel, or that the webhook is current.",
		}},
		{messaging.ErrPostRefused, ownWords()},
		{messaging.ErrUnexpectedStatus, wording{
			brief: "an unexpected service answer",
			full:  "The messaging service answered with a status it does not document. Try the post again.",
		}},
		{messaging.ErrUnreachable, wording{
			brief: "could not reach messaging",
			full:  "The messaging service did not answer. Check the network, then try the announcement again.",
		}},
	}
}

// taskwarriorErrors are Taskwarrior's: finding it, and what it answers.
func taskwarriorErrors() []knownError {
	return []knownError{
		// First, as it wraps Taskwarrior's own answer to the annotation, which
		// would otherwise be told instead.
		{taskwarrior.ErrAnnotateFailed, wording{
			brief: "created, not annotated",
			full: "The task was created but the issue's link could not be added as an annotation: " +
				"`task <id> annotate <url>` adds it.",
		}},
		setUp("Taskwarrior is not installed", taskwarrior.ErrNotInstalled),
		setUp("`task` is not Taskwarrior", taskwarrior.ErrNotTaskwarrior),
		{taskwarrior.ErrTooOld, wording{
			brief: "Taskwarrior is too old",
			full:  "Taskwarrior is too old: 3.5.0 or newer is needed; `workflow doctor` shows the version found.",
		}},
		setUp("Taskwarrior has never run", taskwarrior.ErrNotConfigured),
		{taskwarrior.ErrNothingChanged, wording{
			brief: "nothing changed",
			full:  "Taskwarrior changed nothing: the task is already in that state, or is no longer pending. Refresh.",
		}},
		{taskwarrior.ErrRefused, ownWords()},
		{taskwarrior.ErrBadOutput, wording{
			brief: "unreadable answer",
			full:  "Taskwarrior answered with something other than its JSON; `task export` in a terminal shows what.",
		}},
		{taskwarrior.ErrNoSync, wording{
			brief: "no sync backend",
			full: "No sync backend is set in your taskrc, so there is nowhere to sync. Set one of the sync.* " +
				"settings (task-sync(5)).",
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
		// Its own words are the advice, so they are not told twice.
		{gitrepo.ErrNoIdentity, wording{brief: "git has no user.email", full: noIdentityAdvice(), whole: true}},
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

// noIdentityAdvice is how to give git a user.email, in the shared words.
func noIdentityAdvice() string {
	advice, _ := loop.SetUpAdvice(gitrepo.ErrNoIdentity)

	return advice
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
	words, _ := errorSentence(err)
	if words.brief != "" {
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
// "re-run failed: " — for a notice no open overlay names: the footer clips a
// long sentence, and the lead must survive it.
func (m Model) noticedFailureLedBy(lead string, err error) Model {
	words := []string{markOf(m.marks, err) + " " + lead + inFull(err)}
	if sentence, known := errorSentence(err); known && !sentence.whole {
		words = append(words, ownText(err))
	}

	m = m.noticed(strings.Join(words, "\n"))
	m.notice.failed = !loop.NotSetUp(err)

	return m
}

// noticedGuidance tells, plainly, why a key did nothing and what would. The
// refusal is not a failure, so it takes no mark and no red — red means
// something broke — but its words live where every failure's do: in
// errorSentence, or in the refusal's own text.
func (m Model) noticedGuidance(refusal error) Model {
	return m.noticed(inFull(refusal))
}

// markOf is the mark a cause opens its row with: the not-started mark for
// what was never set up, the failure mark for anything that broke. Shape, not
// color, tells them apart, so they read apart in monochrome as well.
func markOf(marks glyphs, err error) string {
	if loop.NotSetUp(err) {
		return marks.notStarted
	}

	return marks.failed
}

// voice is how a cause is drawn — its mark and the style its sentence takes —
// and the one place the interface decides between the two: what was never set
// up is guidance, plain, with the not-started mark, since nothing was asked and
// nothing refused; anything else broke, and is red with the failure mark.
func voice(sty styles, marks glyphs, err error) (string, lipgloss.Style) {
	if loop.NotSetUp(err) {
		return marks.notStarted, lipgloss.NewStyle()
	}

	return marks.failed, sty.failure
}

// voicedMark is a cause's mark in its voice's style.
func voicedMark(sty styles, marks glyphs, err error) string {
	mark, style := voice(sty, marks, err)

	return style.Render(mark)
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
	return voicedMark(m.styles, m.marks, err) + " " + briefly(err)
}

// unreadRow is a rail's row for a load that did not answer, pointing at the
// detail that tells why: what failed, in the failure voice, or that it is not
// set up, as guidance.
func unreadRow(sty styles, marks glyphs, err error, failed string) string {
	if loop.NotSetUp(err) {
		failed = "not set up"
	}

	return voicedMark(sty, marks, err) + " " + failed + marks.separator + "see detail"
}

// failureLine is a failure on a row of its own with room to spare — an
// overlay's outcome, a field's problem, a run's headline: the red mark and the
// error in full, on the one row. The free form serves an overlay, which draws
// with styles and glyphs but no Model.
func failureLine(sty styles, marks glyphs, err error) string {
	return voicedMark(sty, marks, err) + " " + inFull(err)
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
	mark, style := voice(sty, marks, err)

	words, known := errorSentence(err)
	if !known {
		return style.Render(wrap(mark+" "+ownText(err), width))
	}

	if words.whole {
		return style.Render(wrap(mark+" "+words.full, width))
	}

	return style.Render(wrap(mark+" "+words.full, width)) + "\n" +
		sty.label.Render(wrap(ownText(err), width))
}

// failureBlock is failureBlock for a Model.
func (m Model) failureBlock(err error, width int) string {
	return failureBlock(m.styles, m.marks, err, width)
}

// pinnedProblem is what a form finds wrong with what was typed, drawn under its
// title as its pinned outcome is and wrapped the same way; nothing when
// nothing is.
func pinnedProblem(sty styles, marks glyphs, problem error, width int) []string {
	if problem == nil {
		return nil
	}

	return []string{failureBlock(sty, marks, problem, width), ""}
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
