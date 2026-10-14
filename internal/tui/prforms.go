// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// asking is the composer asking stage's form, on its field focusing, and the
// command that starts it: drawn in the composer or, with accessible forms on,
// handed to the prompts.
func (c prComposer) asking(m Model, stage prStage, focusing int) (Model, tea.Cmd) {
	fields := c.whatFields()
	if stage == prWhere {
		fields = c.whereFields()
	}

	// Its start asks for a size it is already given and for titles drawn
	// from functions none of its fields has: it has no use for either.
	width, rows := composerSize(m)
	form, _ := newHuhForm(m.kit(), prFormKeys(m.keys), width, rows, huh.NewGroup(fields...))

	for range focusing {
		form.NextField()
	}

	c.stage, c.form, c.fields = stage, form, fields
	m.overlay = c

	return m, m.startForm(form, nil, func(err error) tea.Msg { return prAnswered{form: form, err: err} })
}

// composerSize is the columns and rows the composer's form has: the pane's,
// less the body it shows below.
func composerSize(m Model) (int, int) {
	return m.detailWidth(), max(1, m.detailRows()-prBodyPreviewLines)
}

// whereFields ask where the pull request goes: the base, completing to a
// remote branch, and, where the repository has more than one and the body is
// not edited, the template its body starts from.
func (c prComposer) whereFields() []huh.Field {
	base := newQuietInput("Base", "", &c.answers.base).suggesting(c.branches).
		hinted("The branch " + sanitize.Line(c.head) + " merges into.")
	if !c.offersTemplate() {
		return []huh.Field{base}
	}

	options := make([]huh.Option[int], len(c.templates))
	for index, template := range c.templates {
		options[index] = huh.NewOption(sanitize.Line(template.Name), index)
	}

	return []huh.Field{base, huh.NewSelect[int]().Title("Template").Options(options...).Value(&c.answers.template)}
}

// choosesTemplate reports more than one template to choose the body from.
func (c prComposer) choosesTemplate() bool {
	return len(c.templates) > 1
}

// whatFields ask what the pull request says: its title, whether it is a
// draft, and who reviews it, is assigned it, and what labels it carries.
func (c prComposer) whatFields() []huh.Field {
	listed := func(title, description string, value *string) huh.Field {
		return newQuietInput(title, "", value).hinted(description)
	}

	return []huh.Field{
		newQuietInput("Title", "", &c.answers.title),
		huh.NewConfirm().Title("Draft").Affirmative("Yes").Negative("No").Inline(true).Value(&c.answers.draft),
		listed("Reviewers", "Comma-separated; org/team names a team.", &c.answers.reviewers),
		listed("Assignees", "Comma-separated.", &c.answers.assignees),
		listed("Labels", "Comma-separated.", &c.answers.labels),
	}
}

// prAnswered is the composer's form answered as prompts, or the error that
// stopped them.
type prAnswered struct {
	form *huh.Form
	err  error
}

var _ applier = prAnswered{}

// apply moves the composer on from the form its prompts answered. A form
// since closed or replaced moves nothing.
func (msg prAnswered) apply(m Model) (Model, tea.Cmd) {
	composer, open := m.overlay.(prComposer)
	if !open || composer.form != msg.form {
		return m, nil
	}

	if msg.err != nil {
		m.overlay = composer.failed(msg.err)

		return m, nil
	}

	return composer.answered(m)
}

// answered moves on from the form asking now: where it goes to what it
// says, reading the branch again first onto a base chosen; and what it says
// to opening it.
func (c prComposer) answered(m Model) (Model, tea.Cmd) {
	if c.stage == prWhat {
		return c.open(m)
	}

	if !c.edited {
		c = c.withTemplate(c.answers.template)
	}

	onto := readBranchOnto(m.session, m.deps, c)
	if onto == nil {
		return c.asking(m, prWhat, 0)
	}

	c.stage, c.awaiting.onto = prReading, true
	m.overlay = c

	return m, onto
}
