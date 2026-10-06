// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// Trade-off TRADE-30: a comment is written here, in a box drawn inside the
// interface, rather than only in $EDITOR as every other long text is.

// writingMode is the comment composer's mode, as vim has them: in normal mode
// keys are commands, and in insert mode every key types.
type writingMode int

const (
	modeNormal writingMode = iota
	modeInsert
)

// line is the mode as the composer's top line words it.
func (w writingMode) line() string {
	switch w {
	case modeInsert:
		return "-- INSERT --"
	case modeNormal:
		return "-- NORMAL --"
	}

	return "-- NORMAL --"
}

// minCommentRows is the fewest rows the text box keeps; below it the markup's
// cheat line gives its rows up first.
const minCommentRows = 3

// commentDraft is a comment written and not posted, for one issue.
type commentDraft struct {
	issue jira.Key
	text  string
}

// commentDrafts are the drafts kept for the session, at most one per issue.
type commentDrafts []commentDraft

// on is the draft kept for an issue, or "" where none is.
func (d commentDrafts) on(issueKey jira.Key) string {
	index := slices.IndexFunc(d, func(draft commentDraft) bool { return draft.issue == issueKey })
	if index < 0 {
		return ""
	}

	return d[index].text
}

// keeping is the drafts with an issue's replaced by text, or dropped when text
// is blank. It copies rather than change the drafts it was called on.
func (d commentDrafts) keeping(issueKey jira.Key, text string) commentDrafts {
	kept := slices.DeleteFunc(slices.Clone(d), func(draft commentDraft) bool { return draft.issue == issueKey })
	if strings.TrimSpace(text) == "" {
		return kept
	}

	return append(kept, commentDraft{issue: issueKey, text: text})
}

// commentComposer is a comment being written on an issue, in a box drawn in the
// detail pane, with vim's two modes: it opens in normal mode, where i, a, A or
// o start typing and enter shows the comment for a last look; in insert mode
// every key types, and esc goes back to normal mode.
//
// Trade-off TRADE-30: it is the one multi-line text written in the interface
// rather than handed to $EDITOR, which ctrl+o still reaches.
type commentComposer struct {
	marks  glyphs
	styles styles
	issue  jira.Issue
	text   textarea.Model
	mode   writingMode
	markup loop.CommentMarkup
	// quickActions is a comment GitLab reads its slash lines from as
	// commands, which the box says under the markup line.
	quickActions bool
	editor       bool
	// hint is what the empty box says in normal mode: the key that starts
	// typing, as ui.keys binds it.
	hint string
	send sendState
}

var (
	_ editable  = commentComposer{}
	_ pasteable = commentComposer{}
)

// startComment opens the composer on the selected issue, on the draft kept for
// it where there is one.
func (m Model) startComment() (Model, tea.Cmd) {
	selected, ok := m.issues.current()
	if !ok || m.deps.Jira.Comment == nil {
		return m, nil
	}

	markup := loop.CommentMarkupOf(m.cfg.Jira, selected.Key)
	m.overlay = commentComposer{
		marks: m.marks, styles: m.styles, issue: selected, mode: modeNormal,
		text:   newCommentText(m.commentDrafts.on(selected.Key)),
		markup: markup, quickActions: markup == loop.MarkupForgeMarkdown && m.deps.Forge.Kind == forge.KindGitLab,
		editor: m.deps.Editor.Edit != nil,
		hint:   "press " + m.keys.insert.Help().Key + " to write",
	}

	return m, nil
}

// newCommentText is the composer's text box holding draft, its cursor at the
// end. It draws in no hue and its cursor does not blink, and it has no
// clipboard or selection keys: those run a clipboard program outside
// internal/proc, or select text the next key would delete with no undo.
func newCommentText(draft string) textarea.Model {
	box := textarea.New()
	box.Prompt = ""
	box.ShowLineNumbers = false
	box.MaxHeight = 0
	box.CharLimit = 0

	for _, unsafe := range []*key.Binding{
		&box.KeyMap.Paste, &box.KeyMap.CopySelection, &box.KeyMap.SelectAll,
		&box.KeyMap.SelectCharacterForward, &box.KeyMap.SelectCharacterBackward,
		&box.KeyMap.SelectWordForward, &box.KeyMap.SelectWordBackward,
		&box.KeyMap.SelectLineUp, &box.KeyMap.SelectLineDown,
	} {
		unsafe.SetEnabled(false)
	}

	plain := textarea.StyleState{
		Base: lipgloss.NewStyle(), Text: lipgloss.NewStyle(), CursorLine: lipgloss.NewStyle(),
		EndOfBuffer: lipgloss.NewStyle(), Prompt: lipgloss.NewStyle(), Selection: lipgloss.NewStyle(),
		Placeholder: lipgloss.NewStyle().Faint(true),
	}
	box.SetStyles(textarea.Styles{
		Focused: plain, Blurred: plain, Cursor: textarea.CursorStyle{Shape: tea.CursorBlock},
	})
	box.SetValue(draft)
	box.MoveToEnd()
	// The command Focus returns starts the cursor's blink; it is dropped, so no
	// timer runs for the life of the box.
	_ = box.Focus()

	return box
}

// view draws the mode, the issue and the box, with a line of the markup in
// effect under it while there is room. The mode line comes first after any
// pinned failure: the frame clips from the bottom, and layout sizes the box so
// nothing above it is pushed out.
func (c commentComposer) view(width, rows int) (string, string) {
	above, below, height := c.layout(width, rows)
	c = c.fitted(width, height)

	return "Comment on " + shownKey(c.issue.Key), strings.Join(slices.Concat(above, []string{c.text.View()}, below), "\n")
}

// layout is the lines above the box, the lines below it, and the box's height
// in rows rows.
func (c commentComposer) layout(width, rows int) ([]string, []string, int) {
	above := pinnedOutcome(c.styles, c.marks, c.send, "", width)
	above = append(above, c.styles.strong.Render(c.mode.line()),
		wrap(shownKey(c.issue.Key)+" "+c.issue.Summary, width), "")

	below := []string{"", c.styles.label.Render(wrap(markupCheat(c.markup), width))}
	if c.quickActions {
		below = append(below, c.styles.label.Render(wrap(quickActionNote, width)))
	}

	height := rows - rowsOf(above) - rowsOf(below)
	if height < minCommentRows {
		below, height = nil, rows-rowsOf(above)
	}

	return above, below, max(1, height)
}

// rowsOf is how many screen rows lines take, counting each wrapped line of
// an entry that holds several.
func rowsOf(lines []string) int {
	return strings.Count(strings.Join(lines, "\n"), "\n") + 1
}

// fitted is the composer with its box sized to width and height, and the
// placeholder naming the key that starts typing while it is empty in normal
// mode.
func (c commentComposer) fitted(width, height int) commentComposer {
	c.text.SetWidth(max(1, width))
	c.text.SetHeight(height)
	c.text.Placeholder = ""

	if c.mode == modeNormal {
		c.text.Placeholder = c.hint
	}

	return c
}

// quickActionNote warns that GitLab runs a comment's slash lines, such as
// /close, as commands rather than posting them.
const quickActionNote = "GitLab runs a line starting with / as a quick action"

// markupCheat is a line of the markup a comment is written in.
func markupCheat(markup loop.CommentMarkup) string {
	switch markup {
	case loop.MarkupJiraMarkdown:
		return "**bold** *italic* `code` [text](https://…) - list · converted to Jira's markup when posted"
	case loop.MarkupWiki:
		return "*bold* _italic_ {{code}} [text|https://…] * list · Jira's own markup"
	case loop.MarkupForgeMarkdown:
		return "**bold** *italic* `code` [text](https://…) - list · the forge renders it"
	}

	return ""
}

// footer offers starting to type, the preview and closing in normal mode, and
// the one key that leaves insert mode.
func (c commentComposer) footer(keys keyMap) []key.Binding {
	if c.mode == modeInsert {
		return []key.Binding{key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "normal mode"))}
	}

	footer := []key.Binding{
		keys.insert, relabel(keys.confirm, "preview"), relabel(keys.closeOverlay, escClose), keys.openLine,
	}
	if c.editor {
		footer = append(footer, keys.editBody)
	}

	return footer
}

// handleKey answers a key in the mode the composer is in. The box is sized as
// it is drawn first, so the cursor moves over the lines on screen.
func (c commentComposer) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	_, _, height := c.layout(m.detailWidth(), m.detailRows())
	c = c.fitted(m.detailWidth(), height)

	if c.mode == modeInsert {
		m.overlay = c.typed(msg)

		return m, nil
	}

	return c.handleNormalKey(m, msg)
}

// typed answers a key in insert mode: esc leaves it, its cursor stepping back
// onto the last character typed as vim's does, and every other key types.
// It reads esc itself, so a printable key ui.keys moved onto close still types.
func (c commentComposer) typed(msg tea.KeyPressMsg) commentComposer {
	if msg.Code == tea.KeyEscape {
		c.mode = modeNormal
		c.text.SetCursorColumn(c.text.Column() - 1)

		return c
	}

	if text := typedText(msg); text == " " && msg.Text == "" {
		c.text.InsertString(text)
	} else {
		c.text, _ = c.text.Update(msg)
	}

	c.send.err = nil

	return c
}

// handleNormalKey answers a command in normal mode: close, preview, hand the
// draft to the editor, move, or start typing. Any other key does nothing.
func (c commentComposer) handleNormalKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.closeOverlay):
		return c.close(m), nil
	case key.Matches(msg, m.keys.confirm):
		return c.preview(m), nil
	case key.Matches(msg, m.keys.editBody) && c.editor:
		m.overlay = c

		return m, m.deps.Editor.Edit(c.text.Value(), commentHelp(c.markup), func(text string, err error) tea.Msg {
			return textEdited{text: text, err: err}
		})
	}

	m.overlay = c.moved(m.keys, msg)

	return m, nil
}

// moved is the composer after a normal-mode key that moves the cursor or
// starts typing, or as it was for a key that does neither.
func (c commentComposer) moved(keys keyMap, msg tea.KeyPressMsg) commentComposer {
	steps := map[*key.Binding]func(){
		&keys.up:          c.text.CursorUp,
		&keys.down:        c.text.CursorDown,
		&keys.cursorLeft:  func() { c.text.SetCursorColumn(c.text.Column() - 1) },
		&keys.cursorRight: func() { c.text.SetCursorColumn(min(c.text.Column()+1, c.lastColumn())) },
		&keys.insert:      func() { c.mode = modeInsert },
		&keys.appendAfter: func() { c.text.SetCursorColumn(c.text.Column() + 1); c.mode = modeInsert },
		&keys.appendLine:  func() { c.text.CursorEnd(); c.mode = modeInsert },
		&keys.openLine:    func() { c.text.CursorEnd(); c.text.InsertString("\n"); c.mode = modeInsert },
	}

	for binding, step := range steps {
		if key.Matches(msg, *binding) {
			step()
		}
	}

	return c
}

// close closes the composer, keeping what was written as the issue's draft.
func (c commentComposer) close(m Model) Model {
	text := c.text.Value()
	m.commentDrafts = m.commentDrafts.keeping(c.issue.Key, text)
	m = m.closeOverlay()

	if strings.TrimSpace(text) == "" {
		return m
	}

	return m.noticed("draft kept for " + shownKey(c.issue.Key) + "; " + m.keys.comment.Help().Key + " picks it up again")
}

// preview shows the comment for a last look before it is posted, or says there
// is nothing to post.
func (c commentComposer) preview(m Model) Model {
	if strings.TrimSpace(c.text.Value()) == "" {
		m.overlay = c

		return m.noticedGuidance(errEmptyComment)
	}

	m.overlay = commentPreview{marks: c.marks, styles: c.styles, composer: c}

	return m
}

// applyEdit takes the draft back from the editor, in normal mode, or keeps
// why the editor failed under the title with the draft untouched.
func (c commentComposer) applyEdit(m Model, text string, err error) (Model, tea.Cmd) {
	if err != nil {
		c.send = c.send.failed(err)
	} else {
		c.text.SetValue(text)
		c.text.MoveToEnd()
		c.mode, c.send.err = modeNormal, nil
	}

	m.overlay = c

	return m, nil
}

// lastColumn is the column of the cursor line's last character, where vim's l
// stops; a later a is what reaches past it.
func (c commentComposer) lastColumn() int {
	lines := strings.Split(c.text.Value(), "\n")

	return max(0, len([]rune(lines[c.text.Line()]))-1)
}

// pasted types a paste at the cursor in either mode, as vim's bracketed paste
// does.
func (c commentComposer) pasted(m Model, paste tea.PasteMsg) (Model, tea.Cmd) {
	c.text.InsertString(paste.Content)
	c.send.err = nil
	m.overlay = c

	return m, nil
}
