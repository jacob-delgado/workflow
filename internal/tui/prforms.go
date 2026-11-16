// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/quietinput"
)

// asking is the composer asking stage's form, on its field focusing, and the
// command that starts it: drawn in the composer or, with accessible forms on,
// handed to the prompts. Only prAskingWhere asks where the pull request goes;
// any other stage gets the form asking what it says, hidden while the stage
// is a reading one, so a caller passes prAskingWhere or prAskingWhat.
func (c prComposer) asking(m Model, stage prStage, focusing int) (Model, tea.Cmd) {
	fields := c.whatFields()
	if stage == prAskingWhere {
		fields = c.whereFields()
	}

	// Its start asks for a size it is already given and for titles drawn
	// from functions none of its fields has: it has no use for either.
	width, rows := composerSize(m)
	form, _ := newHuhForm(m.kit(), prFormKeys(m.keys), width, rows, huh.NewGroup(fields...))

	for range focusing {
		form.NextField()
	}

	c.flow, c.form, c.fields = c.flow.asking(stage), form, fields
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
	base := quietinput.New("Base", "", &c.answers.base).Suggesting(c.branches).
		Hinted("The branch " + sanitize.Line(c.head) + " merges into.")
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
		return quietinput.New(title, "", value).Hinted(description)
	}

	return []huh.Field{
		quietinput.New("Title", "", &c.answers.title),
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
	if c.flow.stage == prAskingWhat {
		return c.open(m)
	}

	if !c.edited {
		c = c.withTemplate(c.answers.template)
	}

	onto := readBranchOnto(m.session, m.deps, c)
	if onto == nil {
		return c.asking(m, prAskingWhat, 0)
	}

	c.flow = c.flow.readingOnto()
	m.overlay = c

	return m, onto
}

// prStage is where the pull request composer is: reading what it starts
// from, asking where the pull request goes, reading the branch again onto
// the base chosen there, or asking what it says.
type prStage int

const (
	prReadingStarts prStage = iota
	prAskingWhere
	prReadingOnto
	prAskingWhat
)

// prRead is a read the composer waits on before a form opens, a bit of its
// own, so the reads pending are a set: the templates and remote branches,
// the issue's title, the code owners, and the branch read again onto a base
// chosen.
type prRead uint8

const (
	prReadStarts prRead = 1 << iota
	prReadTitle
	prReadReviewers
	prReadOnto
)

// awaited is read when cmd reads it, and no read when there is no command.
func awaited(cmd tea.Cmd, read prRead) prRead {
	if cmd == nil {
		return 0
	}

	return read
}

// prNext is what the composer does once a read answers: wait on the reads
// still pending, ask where the pull request goes, or ask what it says.
type prNext int

const (
	prWaits prNext = iota
	prAsksWhere
	prAsksWhat
)

// prFlow is where the composer is and the reads it waits on there. A form
// opens only once none is pending, so nothing a read finds changes a field
// under the cursor.
type prFlow struct {
	stage   prStage
	pending prRead
}

// readingStarts is the flow of a composer just opened, reading what it
// starts from: the reads pending.
func readingStarts(pending prRead) prFlow {
	return prFlow{stage: prReadingStarts, pending: pending}
}

// reading reports a stage that reads before a form asks, whose mark turns
// as it waits.
func (f prFlow) reading() bool {
	return map[prStage]bool{
		prReadingStarts: true, prAskingWhere: false, prReadingOnto: true, prAskingWhat: false,
	}[f.stage]
}

// next is what the composer does now: wait while a read is pending, and
// otherwise ask what its reading stage leads to, where the pull request goes
// after what it starts from and what it says after the branch onto the base.
// A form already asking asks nothing anew.
func (f prFlow) next() prNext {
	if f.pending != 0 {
		return prWaits
	}

	return map[prStage]prNext{
		prReadingStarts: prAsksWhere, prAskingWhere: prWaits, prReadingOnto: prAsksWhat, prAskingWhat: prWaits,
	}[f.stage]
}

// answered is the flow with read no longer pending.
func (f prFlow) answered(read prRead) prFlow {
	f.pending &^= read

	return f
}

// awaiting is the flow waiting on read as well.
func (f prFlow) awaiting(read prRead) prFlow {
	f.pending |= read

	return f
}

// readingOnto is the flow reading the branch again onto the base the form
// asking where it goes was answered with.
func (f prFlow) readingOnto() prFlow {
	return prFlow{stage: prReadingOnto, pending: f.pending | prReadOnto}
}

// asking is the flow with stage's form open.
func (f prFlow) asking(stage prStage) prFlow {
	f.stage = stage

	return f
}
