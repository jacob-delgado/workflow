// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
)

// hookPickerTitle titles the pane while the hook picker is open.
const hookPickerTitle = "Run a hook"

// preCommit is the hook the picker chooses first, so h then enter runs it as
// h alone once did.
const preCommit = "pre-commit"

// hookPicker chooses which of lefthook's hooks to run by hand: the ones a run
// by hand can stand in for git's, and below them those only git runs, with
// what git hands each. The hooks are read each time it opens, so it always
// shows the configuration as it is now.
type hookPicker struct {
	opened int
	// listing is the hooks lefthook configures, read once as the picker
	// opens; hooks is the choice among those a run by hand can stand in for.
	listing latest.Read[hooks.Configured]
	hooks   pickList[string]
}

var (
	_ opened    = hookPicker{}
	_ steppable = hookPicker{}
	_ clickable = hookPicker{}
)

// openHookPicker opens the picker and starts reading the hooks lefthook
// configures.
func (m Model) openHookPicker() (Model, tea.Cmd) {
	m, opened := m.nextRound()
	m.overlay = hookPicker{opened: opened}
	ctx, list := m.session, m.deps.Hooks.Configured

	return m, func() tea.Msg {
		configured, err := list(ctx)

		return hooksListed{opened: opened, configured: configured, err: err}
	}
}

// hooksListed is the hooks lefthook configures, or why they could not be read.
type hooksListed struct {
	opened     int
	configured hooks.Configured
	err        error
}

var _ applier = hooksListed{}

// apply fills in the picker that asked, choosing pre-commit when it can run,
// and nothing once that picker has closed.
func (msg hooksListed) apply(m Model) (Model, tea.Cmd) {
	picker, open := answerFor[hookPicker](m, msg.opened)
	if !open {
		return m, nil
	}

	runnable := msg.configured.Runnable
	picker.hooks = pickList[string]{items: runnable, selected: max(0, slices.Index(runnable, preCommit))}
	// The picker reads its hooks once, as it opens, before it could begin a
	// round: the listing is round 0's.
	picker.listing, _ = picker.listing.Answered(0, msg.configured, msg.err)

	return m.withAnswered(picker), nil
}

// header is the rows above the picker's list: a prompt and a blank. The view
// draws it and the click measures it.
func (p hookPicker) header() []string {
	return []string{"Run which hook, without committing or pushing?", ""}
}

// view draws the hooks as the listing stands: reading them, why they could
// not be read, or them.
func (p hookPicker) view(kit renderKit, width, rows int) (string, string) {
	failed := func() []string { return []string{kit.failureBlock(p.listing.Err(), width)} }
	lines := drawRead(p.listing, drawnRead[[]string]{
		reading:    func() []string { return []string{"reading lefthook's configuration" + kit.marks.ellipsis} },
		shown:      func() []string { return p.listed(kit, width, rows) },
		failed:     failed,
		failedOver: failed,
	})

	return hookPickerTitle, strings.Join(lines, "\n")
}

// listed draws the hooks to choose from in as many rows as fit, then those
// only git runs, or says why there is nothing to choose.
func (p hookPicker) listed(kit renderKit, width, rows int) []string {
	switch {
	case len(p.hooks.items) == 0 && len(p.gitOnly()) == 0:
		return []string{"lefthook configures no hook here."}
	case len(p.hooks.items) == 0:
		return append([]string{"None of lefthook's hooks here can be run by hand."}, p.gitOnlyLines(kit, width)...)
	default:
		lines := append(p.header(), p.hooks.rows(kit.marks, p.listRows(kit, width, rows), sanitize.Line)...)

		return append(lines, p.gitOnlyLines(kit, width)...)
	}
}

// gitOnly is the hooks only git runs, as listed; none until they are.
func (p hookPicker) gitOnly() []hooks.GitOnly {
	configured, _ := p.listing.Value()

	return configured.GitOnly
}

// listRows is how many rows the hooks to choose from have: what the header
// and the hooks only git runs leave, and at least one.
func (p hookPicker) listRows(kit renderKit, width, rows int) int {
	return max(1, rows-len(p.header())-len(p.gitOnlyLines(kit, width)))
}

// gitOnlyIndent sets off the lines a hook only git runs wraps onto from the
// hook it belongs to.
const gitOnlyIndent = "  "

// gitOnlyLines lists the hooks only git runs, each with what git hands it,
// under a blank and a heading; nothing when there are none.
func (p hookPicker) gitOnlyLines(kit renderKit, width int) []string {
	gitOnly := p.gitOnly()
	if len(gitOnly) == 0 {
		return nil
	}

	lines := []string{"", "Only git runs these:"}

	for _, hook := range gitOnly {
		said := wrap(sanitize.Line(hook.Name)+kit.marks.separator+hook.Reason, width-len(gitOnlyIndent))
		for index, line := range strings.Split(said, "\n") {
			if index > 0 {
				line = gitOnlyIndent + line
			}

			lines = append(lines, kit.styles.label.Render(line))
		}
	}

	return lines
}

// footer offers moving and running once there is a hook to run, and leaving.
func (p hookPicker) footer(keys keyMap) []key.Binding {
	if len(p.hooks.items) == 0 {
		return []key.Binding{keys.closeOverlay}
	}

	return []key.Binding{keys.up, keys.down, relabel(keys.confirm, "run"), keys.closeOverlay}
}

// handleKey moves the choice, runs the chosen hook, or leaves.
func (p hookPicker) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if listed, answered := m.listKey(p, msg); answered {
		return listed, nil
	}

	chosen, found := p.hooks.chosen()
	if !found || !key.Matches(msg, m.keys.confirm) {
		return m, nil
	}

	return m.runChosenHook(chosen)
}

// which names the hook picker.
func (hookPicker) which() overlayKind { return overlayHookPicker }

// openedAs is the round the picker opened in.
func (p hookPicker) openedAs() int { return p.opened }

// step moves the choice of hook by delta.
func (p hookPicker) step(m Model, delta int) Model {
	p.hooks = p.hooks.moved(delta)
	m.overlay = p

	return m
}

// click chooses the hook on a clicked line.
func (p hookPicker) click(m Model, line int) (Model, tea.Cmd) {
	header := len(p.header())
	p.hooks = p.hooks.clicked(line-header, p.listRows(m.kit(), m.detailWidth(), m.detailRows()))
	m.overlay = p

	return m, nil
}

// runChosenHook runs one hook as git would, without committing or pushing.
func (m Model) runChosenHook(hook string) (Model, tea.Cmd) {
	shown := sanitize.Line(hook)
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would run the " + shown + " hook"), nil
	}

	ctx, run := m.session, m.deps.Hooks.Run

	return m.startRun(hookRun(shown), func() (proc.Output, error) { return run(ctx, hook) }, nil)
}

// hookRun is a run of one hook alone, which a check can fail.
func hookRun(shown string) runKind {
	return runKind{title: shown, refusal: "the " + shown + " hook failed"}
}
