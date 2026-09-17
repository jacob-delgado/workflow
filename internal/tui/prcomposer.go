// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/forge"
)

// prLabelWidth is the columns a pull request field's marker, label, prompt and
// the cursor after the text take.
const prLabelWidth = 12

// prBodyPreviewLines is how much of a pull request body the composer shows.
const prBodyPreviewLines = 12

// prBodyHelp is what the editor shows below a pull request body.
const prBodyHelp = "Write the pull request description above this line. Markdown renders on both\n" +
	"GitHub and GitLab."

// errNoTitle and errNoBase report a change missing what the forge needs.
var (
	errNoTitle = errors.New("a title is required")
	errNoBase  = errors.New("a base branch to merge into is required")
)

// Pull request composer fields, in the order tab moves through them.
const (
	prFieldTitle = iota
	prFieldBase
	prFields
)

// prComposer is a pull request about to be opened, started from the branch's
// commits and the repository's template, all of it editable.
type prComposer struct {
	marks     glyphs
	styles    styles
	title     textinput.Model
	base      textinput.Model
	focus     int
	head      string
	templates []forge.Template
	template  int
	subjects  []string
	issueKey  string
	issueURL  string
	body      string
	draft     bool
	edited    bool
	vocab     reviewVocab
	problem   error
	sending   bool
}

var _ overlay = prComposer{}

// prDraft is a pull request the composer was filled with, kept for the session
// so a push that fails, or an esc, does not throw the work away.
type prDraft struct {
	branch, title, base, body string
	template                  int
	draft, edited             bool
}

// snapshot is the composer's editable state, to reopen on.
func (c prComposer) snapshot() prDraft {
	return prDraft{
		branch: c.head, title: c.title.Value(), base: c.base.Value(), body: c.body,
		template: c.template, draft: c.draft, edited: c.edited,
	}
}

// restore fills the composer from a kept draft.
func (c prComposer) restore(draft prDraft) prComposer {
	c.title.SetValue(draft.title)
	c.base.SetValue(draft.base)
	c.body, c.template, c.draft, c.edited = draft.body, draft.template, draft.draft, draft.edited

	return c
}

// openPullRequestComposer proposes a pull request for the branch.
func (m Model) openPullRequestComposer() (Model, tea.Cmd) {
	branch := m.branch.branch

	subjects := make([]string, 0, len(branch.Commits))
	for _, commit := range branch.Commits {
		subjects = append(subjects, commit.Subject)
	}

	issueKey, _ := convention.IssueKey(branch.Name)
	issue, _ := m.issues.find(issueKey)

	composer := prComposer{
		marks: m.marks, styles: m.styles,
		title: newInput(convention.PullRequestTitle(subjects, issueKey, issue.Summary)),
		base:  newInput(strings.TrimPrefix(branch.Base, "origin/")), focus: prFieldTitle, head: branch.Name,
		subjects: subjects, issueKey: issueKey, issueURL: m.browseURL(issueKey), vocab: m.vocab,
	}
	composer.base.Blur()

	if m.deps.Forge.Templates != nil {
		composer.templates = m.deps.Forge.Templates()
	}

	composer = composer.withTemplate(0)
	if m.prDraft.branch == branch.Name {
		composer = composer.restore(m.prDraft)
	}

	m.overlay = composer

	return m, nil
}

// browseURL links an issue, when there is an issue and a way to link it.
func (m Model) browseURL(issueKey string) string {
	if issueKey == "" || m.deps.Jira.BrowseURL == nil {
		return ""
	}

	return m.deps.Jira.BrowseURL(issueKey)
}

// withTemplate starts the body from a template: the repository's, or none.
func (c prComposer) withTemplate(index int) prComposer {
	c.template = index

	text := ""
	if index < len(c.templates) {
		text = c.templates[index].Body
	}

	c.body = convention.PullRequestBody(text, c.subjects, c.issueKey, c.issueURL)

	return c
}

// view shows every part of the pull request as it will be opened.
func (c prComposer) view(width, _ int) (string, string) {
	c.title.Width, c.base.Width = max(1, width-prLabelWidth), max(1, width-prLabelWidth)

	checkbox := map[bool]string{false: "[ ]", true: "[x]"}[c.draft]
	lines := pinnedOutcome(c.styles, c.marks, c.sending, "opening", c.problem, width)
	lines = append(lines,
		c.marks.marker(c.focus == prFieldTitle)+"title  "+c.title.View(),
		c.marks.marker(c.focus == prFieldBase)+"base   "+c.base.View(),
		"  head   "+c.head,
		"  "+c.templateName()+c.marks.separator+checkbox+" draft",
		"",
	)

	body := strings.Split(wrap(strings.TrimRight(c.body, "\n"), width), "\n")
	lines = append(lines, body[:min(len(body), prBodyPreviewLines)]...)

	return "Open " + c.vocab.noun, strings.Join(lines, "\n")
}

// templateName names the template in use, and how many there are to choose
// from.
func (c prComposer) templateName() string {
	if len(c.templates) == 0 {
		return "no template in this repository"
	}

	return "template " + c.templates[c.template].Name +
		" (" + strconv.Itoa(c.template+1) + " of " + strconv.Itoa(len(c.templates)) + ")"
}

// footer offers every part that can be changed, opening, and leaving.
func (c prComposer) footer(keys keyMap) []key.Binding {
	if c.sending {
		return []key.Binding{keys.interrupt}
	}

	bindings := []key.Binding{keys.nextField}
	if c.canNextTemplate() {
		bindings = append(bindings, keys.nextTemplate)
	}

	return append(bindings, keys.toggleDraft, keys.editBody, relabel(keys.confirm, "open"),
		relabel(keys.closeOverlay, "discard"))
}

// canNextTemplate reports another template to cycle to that would not overwrite
// an edited body: ctrl+t is offered only when it can do something safe.
func (c prComposer) canNextTemplate() bool {
	return len(c.templates) > 1 && !c.edited
}

// handleKey answers a key while the pull request is composed.
func (c prComposer) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case c.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		m.prDraft = c.snapshot()

		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return c.open(m)
	case key.Matches(msg, m.keys.editBody):
		m.overlay = c

		return m, c.editBody(m)
	case key.Matches(msg, m.keys.nextTemplate) && c.canNextTemplate():
		c = c.withTemplate((c.template + 1) % len(c.templates))
	case key.Matches(msg, m.keys.toggleDraft):
		c.draft = !c.draft
	case key.Matches(msg, m.keys.nextField, m.keys.prevField):
		c = c.focusOn((c.focus + 1) % prFields)
	default:
		c = c.typed(msg)
	}

	m.overlay = c

	return m, nil
}

// focusOn moves focus to a field, and the text cursor with it.
func (c prComposer) focusOn(field int) prComposer {
	c.focus = field

	c.title.Blur()
	c.base.Blur()

	if field == prFieldBase {
		c.base.Focus()
	} else {
		c.title.Focus()
	}

	return c
}

// typed hands a key to the field with focus.
func (c prComposer) typed(msg tea.KeyMsg) prComposer {
	if c.focus == prFieldBase {
		c.base, _ = c.base.Update(msg)
	} else {
		c.title, _ = c.title.Update(msg)
	}

	c.problem = nil

	return c
}

// editBody opens the editor on the body.
func (c prComposer) editBody(m Model) tea.Cmd {
	if m.deps.Editor.Edit == nil {
		return nil
	}

	return m.deps.Editor.Edit(c.body, prBodyHelp, func(text string, err error) tea.Msg {
		return prBodyEdited{text: text, err: err}
	})
}

// prBodyEdited is a pull request body back from the editor.
type prBodyEdited struct {
	text string
	err  error
}

// apply puts the body in the composer, if it is still open.
func (msg prBodyEdited) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open {
		return m, nil
	}

	if msg.err != nil {
		composer.problem = msg.err
	} else {
		composer.body, composer.edited = msg.text, true
	}

	m.overlay = composer

	return m, nil
}

// request is the pull request the composer describes.
func (c prComposer) request() forge.NewPullRequest {
	return forge.NewPullRequest{
		Title: strings.TrimSpace(c.title.Value()), Body: c.body, Head: c.head,
		Base: strings.TrimSpace(c.base.Value()), Draft: c.draft,
	}
}

// open opens the pull request — pushing the branch first when origin does not
// have every commit, and opening only if the push succeeded.
func (c prComposer) open(m Model) (Model, tea.Cmd) {
	request := c.request()

	switch {
	case request.Title == "":
		c.problem = errNoTitle
	case request.Base == "":
		c.problem = errNoBase
	case m.dryRun:
		return m.closeOverlay().noticed(c.dryRunNotice(request, m.branch.branch.Pushed(), m.issueToLink())), nil
	case m.branch.branch.Pushed():
		return c.create(m)
	default:
		m.prDraft = c.snapshot()

		return m.startPush(func(pushed Model) (Model, tea.Cmd) {
			reload := pushed.loadBranch()
			pushed, create := c.create(pushed)

			return pushed, tea.Batch(reload, create)
		})
	}

	m.overlay = c

	return m, nil
}

// dryRunNotice says what opening the pull request would do, including the push
// that enter does first when origin does not have every commit.
func (c prComposer) dryRunNotice(request forge.NewPullRequest, pushed bool, linkTo string) string {
	open := "open \"" + request.Title + "\" from " + request.Head + " into " + request.Base
	if linkTo != "" {
		open += " and link it on " + linkTo
	}

	if pushed {
		return "dry run: would " + open
	}

	return "dry run: would push " + request.Head + ", then " + open
}

// create asks the forge to open the pull request.
func (c prComposer) create(m Model) (Model, tea.Cmd) {
	c.sending, c.problem = true, nil
	m.overlay = c
	create, request := m.deps.Forge.CreatePullRequest, c.request()

	return m, func() tea.Msg {
		pull, err := create(request)

		return pullCreated{pull: pull, err: err}
	}
}

// pullCreated reports how opening a pull request went.
type pullCreated struct {
	pull forge.PullRequest
	err  error
}

// apply shows the new pull request and starts on its CI, or keeps the composer
// open with the forge's reason.
func (msg pullCreated) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		composer, open := m.overlay.(prComposer)
		if open {
			composer.sending, composer.problem = false, msg.err
			m.overlay = composer
		}

		return m, nil
	}

	m = m.noticed(m.marks.done + " opened " + m.vocab.sigil + strconv.Itoa(msg.pull.Number) + " " + msg.pull.URL)
	m.review = reviewState{pull: msg.pull, found: true, loaded: true}
	m.prDraft = prDraft{}

	cmds := tea.Batch(m.checkCI(), m.loadAuthor())

	// With a Jira issue to link it to, offer to add the link before closing, so
	// the team that watches Jira learns of the pull request.
	if issueKey := m.issueToLink(); issueKey != "" {
		m.overlay = issueLinker{marks: m.marks, styles: m.styles, vocab: m.vocab, issueKey: issueKey, pull: msg.pull}

		return m, cmds
	}

	return m.closeOverlay(), cmds
}
