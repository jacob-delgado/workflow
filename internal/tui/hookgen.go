// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/jacob-delgado/workflow/internal/hooks"
)

// hookgenState is whether generating a lefthook configuration was offered this
// session, which happens at most once.
type hookgenState struct {
	offered bool
}

// findHooks is the command that looks for hooks lefthook does not manage.
func (m Model) findHooks() tea.Cmd {
	existing := m.deps.Hooks.Existing
	if existing == nil || m.deps.Hooks.Write == nil {
		return nil
	}

	return func() tea.Msg {
		found, configured := existing()

		return hooksFound{hooks: found, configured: configured}
	}
}

// hooksFound is what the hooks directory holds.
type hooksFound struct {
	hooks      []hooks.GitHook
	configured bool
}

// apply offers a lefthook configuration for hooks that predate lefthook — once,
// and never over something already open.
func (msg hooksFound) apply(m Model) (Model, tea.Cmd) {
	if msg.configured || len(msg.hooks) == 0 || m.hookgen.offered || m.overlay != nil {
		return m, nil
	}

	m.hookgen.offered = true
	m.overlay = hookgenOffer{marks: m.marks, hooks: msg.hooks, generated: hooks.Structured(msg.hooks)}

	return m, nil
}

// hookgenOffer is a lefthook configuration offered for a repository's hooks.
type hookgenOffer struct {
	marks     glyphs
	hooks     []hooks.GitHook
	generated hooks.Generated
	sending   bool
	problem   error
}

var _ overlay = hookgenOffer{}

// view lists the hooks found and the configuration that would run them.
func (o hookgenOffer) view(width, _ int) (string, string) {
	lines := []string{
		wrap("Found "+plural(len(o.hooks), "hook")+" in .git/hooks that lefthook does not manage:", width), "",
	}

	for _, hook := range o.hooks {
		count := strconv.Itoa(strings.Count(strings.TrimRight(hook.Script, "\n"), "\n") + 1)
		lines = append(lines, "  "+hook.Name+o.marks.separator+count+" lines")
	}

	// The configuration is clipped rather than wrapped: a YAML line broken in
	// two is not the YAML that will be written.
	lines = append(lines, "", "A lefthook.yml that runs them:", "")
	lines = append(lines, strings.Split(strings.TrimRight(o.generated.Config, "\n"), "\n")...)

	if scripts := len(o.generated.Scripts); scripts > 0 {
		lines = append(lines, "", wrap(plural(scripts, "hook")+" kept whole as scripts under .lefthook", width))
	}

	lines = append(lines, "", wrap("lefthook install then keeps the old hooks as .git/hooks/*.old", width))

	switch {
	case o.sending:
		lines = append(lines, "", "writing"+o.marks.ellipsis)
	case o.problem != nil:
		lines = append(lines, "", o.marks.failed+" "+o.problem.Error())
	}

	return "No lefthook configuration", strings.Join(lines, "\n")
}

// footer offers writing it, writing every hook as a script, or not.
func (o hookgenOffer) footer(keys keyMap) []key.Binding {
	if o.sending {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "write lefthook.yml"), keys.verbatim, relabel(keys.closeOverlay, "skip")}
}

// handleKey answers a key while the offer is open.
func (o hookgenOffer) handleKey(m Model, msg tea.KeyMsg) (Model, tea.Cmd) {
	switch {
	case o.sending:
		return m, nil
	case key.Matches(msg, m.keys.closeOverlay):
		return m.closeOverlay(), nil
	case key.Matches(msg, m.keys.confirm):
		return o.write(m, o.generated)
	case key.Matches(msg, m.keys.verbatim):
		return o.write(m, hooks.Verbatim(o.hooks))
	default:
		return m, nil
	}
}

// write writes the configuration and installs lefthook.
func (o hookgenOffer) write(m Model, generated hooks.Generated) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would write lefthook.yml and " +
			plural(len(generated.Scripts), "script") + ", then install lefthook"), nil
	}

	o.sending, o.problem = true, nil
	m.overlay = o
	write := m.deps.Hooks.Write

	return m, func() tea.Msg { return hooksWritten{err: write(generated)} }
}

// hooksWritten reports how writing the configuration went.
type hooksWritten struct {
	err error
}

// apply closes the offer once written, or keeps it open with the reason.
func (msg hooksWritten) apply(m Model) (Model, tea.Cmd) {
	if msg.err != nil {
		offer, open := m.overlay.(hookgenOffer)
		if open {
			offer.sending, offer.problem = false, msg.err
			m.overlay = offer
		}

		return m, nil
	}

	return m.closeOverlay().noticed(m.marks.done + " wrote lefthook.yml and installed lefthook"), nil
}
