// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// prBodyPreviewLines is how much of a pull request body the composer shows.
const prBodyPreviewLines = 12

// prBodyHelp is what the editor shows below a pull request body, naming it by
// the forge's own noun.
func prBodyHelp(vocab reviewVocab) string {
	return "Write the " + vocab.noun + " description above this line. Markdown renders on both\n" +
		"GitHub and GitLab."
}

// errNoTitle and errNoBase report a change missing what the forge needs.
var (
	errNoTitle = errors.New("a title is required")
	errNoBase  = errors.New("a base branch to merge into is required")
)

// prStage is where the pull request composer is: reading what it starts
// from, asking where the pull request goes, or asking what it says.
type prStage int

const (
	prReading prStage = iota
	prWhere
	prWhat
)

// prAnswers is what the composer's forms are answered with, each bound to its
// Huh field. Every copy of the composer shares it, as they share the form.
type prAnswers struct {
	title, base, reviewers, assignees, labels string
	template                                  int
	draft                                     bool
}

// prAwaiting is the reads the composer waits on before its form opens: the
// templates and remote branches, the issue's title, the code owners, and the
// branch read again onto a base chosen.
type prAwaiting struct {
	starts, title, reviewers, onto bool
}

// ready reports every read answered.
func (a prAwaiting) ready() bool {
	return !a.starts && !a.title && !a.reviewers && !a.onto
}

// prComposer is a pull request about to be opened, started from the branch's
// commits and the repository's template, all of it editable: once what it
// starts from is read, a Huh form asks where it goes, the base and the
// template, and then another what it says, the title, whether it is a draft,
// and who reviews it. The body is handed to the editor.
type prComposer struct {
	stage    prStage
	awaiting prAwaiting
	form     *huh.Form
	// fields are the open form's fields, in order: previous-field on the
	// first of what it says goes back to the form before.
	fields    []huh.Field
	answers   *prAnswers
	head      string
	branches  []string
	templates []forge.Template
	// templatesRead reports the repository's templates read.
	templatesRead bool
	// proposedFrom is what the title and body were proposed from, which a
	// template chosen later, or the issue read later, proposes from again.
	proposedFrom loop.DraftInput
	// draftedOnto is the base proposedFrom's commits were read past: the
	// branch's own at first, and each base chosen after it.
	draftedOnto string
	body        string
	edited      bool
	// reviewersSettled reports reviewers typed, or filled from the owners,
	// which a read of the owners then leaves alone. reviewersProposed is what
	// the owners filled, so a base chosen later reads them again only while
	// they still stand untyped over.
	reviewersSettled  bool
	reviewersProposed string
	// opened is the count of overlays opened when this one opened, so the
	// reads made for it land in it alone.
	opened int
	vocab  reviewVocab
	send   sendState
}

var (
	_ editable             = prComposer{}
	_ failable[prComposer] = prComposer{}
	_ pasteable            = prComposer{}
	_ relaying             = prComposer{}
	_ waiting              = prComposer{}
)

// openPullRequestComposer proposes a pull request for the branch, or reopens
// the draft kept for it, and reads what it starts from: the templates and the
// remote branches, an issue the list does not hold for the title, and the
// code owners for the reviewers, unless a kept draft already says who
// reviews. Its form opens once every read has answered, so nothing it reads
// changes a field under the cursor.
func (m Model) openPullRequestComposer() (Model, tea.Cmd) {
	branch := m.branch.branch
	issueKey, _ := m.branch.issue(m.cfg.Jira.Project)
	issue, listed := m.issues.find(issueKey)
	proposed := m.proposePullRequest(branch, issueKey, issue.Summary)

	m, proposed.opened = m.opening()
	if m.prDraft.branch == branch.Name {
		proposed = proposed.restore(m.prDraft)
	}

	var reviewers, title tea.Cmd
	if !proposed.reviewersSettled {
		reviewers = readReviewers(m.session, m.deps, proposed)
	}

	if !listed {
		title = readTitleIssue(m.session, m.deps, proposed)
	}

	starts := readPullRequestStarts(m.session, m.deps, proposed.opened)
	proposed.awaiting = prAwaiting{starts: starts != nil, title: title != nil, reviewers: reviewers != nil}

	m, opening := proposed.settled(m)

	return m, tea.Batch(starts, title, reviewers, opening)
}

// proposePullRequest is the composer filled from the branch's commits, the
// issue it names and the repository's first template.
func (m Model) proposePullRequest(branch gitrepo.Branch, issueKey jira.Key, summary string) prComposer {
	proposedFrom := loop.DraftInput{
		Subjects: loop.Subjects(branch.Commits), IssueKey: issueKey, IssueSummary: summary,
		IssueURL: browseURL(m.deps, issueKey), TitleSource: convention.TitleSource(m.cfg.PullRequest.TitleSource),
	}
	title, _ := loop.Draft(proposedFrom)

	composer := prComposer{
		answers: &prAnswers{title: title, base: branch.BaseName()},
		head:    branch.Name, proposedFrom: proposedFrom, draftedOnto: branch.BaseName(), vocab: m.vocab,
	}
	composer.templatesRead = m.deps.Forge.Templates == nil

	return composer.withTemplate(0)
}

// withTemplate starts the body from a template: the repository's, or none.
func (c prComposer) withTemplate(index int) prComposer {
	c.answers.template = index

	from := c.proposedFrom
	if index < len(c.templates) {
		from.Template = c.templates[index].Body
	}

	_, c.body = loop.Draft(from)

	return c
}

// settled is the composer as the reads answered so far leave it: still
// reading while any is out, and otherwise asking, where it goes on the way
// in and what it says once a base is chosen.
func (c prComposer) settled(m Model) (Model, tea.Cmd) {
	switch {
	case !c.awaiting.ready():
		c.stage = prReading
	case c.stage == prReading && c.form != nil:
		return c.asking(m, prWhat, 0)
	case c.stage == prReading:
		return c.asking(m, prWhere, 0)
	}

	m.overlay = c

	return m, nil
}

// view shows every part of the pull request as it will be opened: the form
// asking now, or what is being read, then where it goes and the body.
func (c prComposer) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedOutcome(c.send, "opening", width)

	if c.stage == prReading {
		lines = append(lines, kit.turned+" reading what the "+c.vocab.noun+" starts from"+kit.marks.ellipsis)
	} else {
		lines = append(lines, c.form.View())
	}

	lines = append(lines, "", kit.styles.label.Render(c.route(kit)), kit.styles.label.Render(c.summary(kit)), "")
	body := strings.Split(wrap(strings.TrimRight(c.body, "\n"), width), "\n")
	lines = append(lines, body[:min(len(body), prBodyPreviewLines)]...)

	return "Open " + c.vocab.noun, strings.Join(lines, "\n")
}

// route says what the pull request goes from and into.
func (c prComposer) route(kit renderKit) string {
	return sanitize.Line(c.head) + kit.marks.arrow + sanitize.Line(strings.TrimSpace(c.answers.base))
}

// summary says the template the body starts from, and whether the pull
// request is a draft.
func (c prComposer) summary(kit renderKit) string {
	if c.answers.draft {
		return c.templateName(kit) + kit.marks.separator + "draft"
	}

	return c.templateName(kit)
}

// templateName names the template in use, and how many there are to choose
// from.
func (c prComposer) templateName(kit renderKit) string {
	switch {
	case !c.templatesRead:
		return "template " + kit.reading()
	case len(c.templates) == 0:
		return "no template in this repository"
	}

	return "template " + sanitize.Line(c.templates[c.answers.template].Name) +
		" (" + strconv.Itoa(c.answers.template+1) + " of " + strconv.Itoa(len(c.templates)) + ")"
}

// waits reports the composer reading what it starts from, whose mark turns
// as it waits.
func (c prComposer) waits() bool {
	return c.stage == prReading
}

// footer offers what the field with the keyboard answers, the composer's own
// keys, going on or opening, and leaving.
func (c prComposer) footer(keys keyMap) []key.Binding {
	switch {
	case c.send.sending:
		return []key.Binding{keys.interrupt}
	case c.stage == prReading:
		return []key.Binding{relabel(keys.closeOverlay, escClose)}
	}

	bindings := formKeys(c.form)
	if c.stage == prWhat && c.focused() == 0 {
		bindings = append(bindings, relabel(keys.prevField, escBack))
	}

	if c.offersTemplate() {
		bindings = append(bindings, keys.nextTemplate)
	}

	onward := "open"
	if c.stage == prWhere {
		onward = "next"
	}

	return append(bindings, keys.toggleDraft, keys.editBody, relabel(keys.confirm, onward),
		relabel(keys.closeOverlay, escClose))
}

// offersTemplate reports another template the body can start from, which
// would not overwrite an edited body: the form offers a choice of them, and
// ctrl+t moves on to the next, only when that is safe.
func (c prComposer) offersTemplate() bool {
	return c.choosesTemplate() && !c.edited
}

// handleKey answers a key while the pull request is composed: esc keeps the
// draft and closes; apply goes on from the form or opens; the composer's own
// keys edit the body, cycle the template and turn the draft; previous-field on
// the first of what it says goes back to where it goes; and every other key
// is the form's.
func (c prComposer) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case c.send.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.prDraft = c.snapshot()

		return m.closeOverlay().noticedDraftKept(m.keys.newPullRequest), nil
	case c.stage == prReading:
		return m, nil
	case key.Matches(msg, m.keys.editBody):
		m.overlay = c

		return m, c.editBody(m)
	}

	return c.formKey(m, msg)
}

// formKey answers a key the open form has: apply goes on, the template and
// the draft change in place, previous-field on the first of what it says goes
// back, and every other key is the form's own.
func (c prComposer) formKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	c.send.err = nil

	switch {
	case key.Matches(msg, m.keys.confirm):
		return c.answered(m)
	case key.Matches(msg, m.keys.nextTemplate) && c.offersTemplate():
		c = c.withTemplate(around(c.answers.template, len(c.templates)).next())

		return c.asking(m, c.stage, c.focused())
	case key.Matches(msg, m.keys.toggleDraft):
		c.answers.draft = !c.answers.draft

		return c.asking(m, c.stage, c.focused())
	case c.stage == prWhat && key.Matches(msg, m.keys.prevField) && c.focused() == 0:
		return c.asking(m, prWhere, 0)
	}

	m.overlay = c

	return c.relay(m, msg)
}

// focused is how many fields into the open form the keyboard is.
func (c prComposer) focused() int {
	return max(0, slices.Index(c.fields, c.form.GetFocusedField()))
}

// resized fits the open form, while one is, to the pane at the screen's new
// size.
func (c prComposer) resized(m Model) (Model, tea.Cmd) {
	if c.form == nil {
		return m, nil
	}

	width, rows := composerSize(m)

	return m, fitForm(c.form, width, rows)
}

// relay hands a message to the open form: a reviewer typed marks the
// reviewers settled, which a read of the owners then leaves alone, and a
// template chosen starts the body from it.
func (c prComposer) relay(m Model, msg tea.Msg) (Model, tea.Cmd) {
	if c.form == nil {
		return m, nil
	}

	reviewers, template := c.answers.reviewers, c.answers.template
	cmd := relayToForm(c.form, msg)
	c.reviewersSettled = c.reviewersSettled || c.answers.reviewers != reviewers

	if c.answers.template != template {
		c = c.withTemplate(c.answers.template)
	}

	m.overlay = c

	return m, cmd
}

// which names the pull request composer.
func (prComposer) which() overlayKind { return overlayPRComposer }

// pasted types a paste into the field with focus, as typing it would.
func (c prComposer) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	if c.send.sending || c.stage == prReading {
		return m, nil
	}

	return c.relay(m, paste)
}

// editBody opens the editor on the body.
func (c prComposer) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(c.body, prBodyHelp(c.vocab), func(text string, err error) tea.Msg {
		return textEdited{text: text, err: err}
	})
}

// applyEdit puts the body back in the composer, or records why the editor
// failed. A form asking where the pull request goes asks again without its
// choice of template, which would now overwrite the body.
func (c prComposer) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		c.send = c.send.failed(err)
		m.overlay = c

		return m, nil
	}

	c.body, c.edited = text, true
	if c.stage == prWhere && c.choosesTemplate() {
		return c.asking(m, prWhere, 0)
	}

	m.overlay = c

	return m, nil
}

// failed is the composer kept open with the reason the pull request did not open.
func (c prComposer) failed(err error) prComposer {
	c.send = c.send.failed(err)

	return c
}
