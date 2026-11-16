// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/jacob-delgado/workflow/internal/hooks"
	"github.com/jacob-delgado/workflow/internal/tui/latest"
	"github.com/jacob-delgado/workflow/internal/tui/send"
)

// hookgenState holds the hooks lefthook does not manage, as the latest look
// at them found them, at start and on each Commits refresh, so that pane can
// offer a configuration for them.
type hookgenState struct {
	read latest.Read[[]hooks.GitHook]
}

// unmanaged is the hooks lefthook does not manage, as last found or told.
func (s hookgenState) unmanaged() []hooks.GitHook {
	found, _ := s.read.Value()

	return found
}

// lookingForHooks begins a look for hooks lefthook does not manage,
// superseding any still out, and is the command that looks; with nothing to
// look or to write with, it begins nothing.
func (m Model) lookingForHooks() (Model, tea.Cmd) {
	if m.deps.Hooks.Existing == nil || m.deps.Hooks.Write == nil {
		return m, nil
	}

	read, round := m.hookgen.read.Begin()
	m.hookgen.read = read

	return m, findHooks(m.session, m.deps, round)
}

// findHooks is the command that looks for hooks lefthook does not manage,
// under ctx, in round.
func findHooks(ctx context.Context, deps Deps, round latest.Round) tea.Cmd {
	existing := deps.Hooks.Existing
	if existing == nil || deps.Hooks.Write == nil {
		return nil
	}

	return func() tea.Msg {
		found, configured := existing(ctx)

		return hooksFound{hooks: found, configured: configured, round: round}
	}
}

// hooksFound is what the hooks directory holds, as the look begun in round
// found it.
type hooksFound struct {
	hooks      []hooks.GitHook
	configured bool
	round      latest.Round
}

var _ applier = hooksFound{}

// apply stores hooks that predate lefthook, so the Commits pane can offer to
// generate a configuration for them — rather than seizing the first screen —
// and forgets them once lefthook is configured, however that came about. A
// look a later one superseded, or one begun before a configuration was
// written, read the hooks as they were before, and is dropped.
func (msg hooksFound) apply(m Model) (Model, tea.Cmd) {
	found := msg.hooks
	if msg.configured {
		found = nil
	}

	if read, taken := m.hookgen.read.Answered(msg.round, found, nil); taken {
		m.hookgen.read = read
	}

	return m, nil
}

// openHookgen opens the lefthook offer, in a round of its own, rebuilt from
// the stored hooks so it reopens after a skip.
func (m Model) openHookgen() (Model, tea.Cmd) {
	found := m.hookgen.unmanaged()
	m, opened := m.nextRound()
	m.overlay = hookgenOffer{hooks: found, generated: hooks.Structured(found), opened: opened}

	return m, nil
}

// hookgenOffer is a lefthook configuration offered for a repository's hooks.
type hookgenOffer struct {
	hooks     []hooks.GitHook
	generated hooks.Generated
	// opened is the round the offer opened in, which its write carries back.
	opened int
	send   send.State
}

var _ sender[hookgenOffer] = hookgenOffer{}

// view lists the hooks found and the configuration that would run them, its
// outcome pinned under the title so a long refusal is seen, not clipped.
func (o hookgenOffer) view(kit renderKit, width, _ int) (string, string) {
	lines := kit.pinnedSend(o.send, "writing", width)
	lines = append(lines,
		wrap("Found "+plural(len(o.hooks), "hook")+" in .git/hooks that lefthook does not manage:", width), "",
	)

	for _, hook := range o.hooks {
		count := strconv.Itoa(strings.Count(strings.TrimRight(hook.Script, "\n"), "\n") + 1)
		lines = append(lines, "  "+hook.Name+kit.marks.separator+count+" lines")
	}

	// The configuration is clipped rather than wrapped: a YAML line broken in
	// two is not the YAML that will be written.
	lines = append(lines, "", "A lefthook.yml that runs them:", "")
	lines = append(lines, strings.Split(strings.TrimRight(o.generated.Config, "\n"), "\n")...)

	if scripts := len(o.generated.Scripts); scripts > 0 {
		lines = append(lines, "", wrap(plural(scripts, "hook")+" kept whole as scripts under .lefthook", width))
	}

	lines = append(lines, "", wrap("lefthook install then keeps the old hooks as .git/hooks/*.old", width))

	return "No lefthook configuration", strings.Join(lines, "\n")
}

// footer offers writing it, writing every hook as a script, or not.
func (o hookgenOffer) footer(keys keyMap) []key.Binding {
	if o.send.InFlight() {
		return []key.Binding{keys.interrupt}
	}

	return []key.Binding{relabel(keys.confirm, "write lefthook.yml"), keys.verbatim, relabel(keys.closeOverlay, escSkip)}
}

// handleKey answers a key while the offer is open.
func (o hookgenOffer) handleKey(m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch {
	case o.send.InFlight():
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

// which names the lefthook offer.
func (hookgenOffer) which() overlayKind { return overlayHookgenOffer }

// openedAs is the round the offer opened in, which its write carries back.
func (o hookgenOffer) openedAs() int { return o.opened }

// sent is the configuration's write.
func (o hookgenOffer) sent() send.State { return o.send }

// withSend is the offer with its write as state.
func (o hookgenOffer) withSend(state send.State) hookgenOffer {
	o.send = state

	return o
}

// write writes the configuration and installs lefthook. handleKey takes no
// key while a write is out, so Sent always sends.
func (o hookgenOffer) write(m Model, generated hooks.Generated) (Model, tea.Cmd) {
	if m.dryRun {
		return m.closeOverlay().noticed("dry run: would write lefthook.yml and " +
			plural(len(generated.Scripts), "script") + ", then install lefthook"), nil
	}

	o.send, _ = o.send.Sent()
	m.overlay = o
	ctx, write, existing, opened := m.session, m.deps.Hooks.Write, m.deps.Hooks.Existing, o.opened

	return m, func() tea.Msg {
		err := write(ctx, generated)
		if err == nil {
			return hooksWritten{opened: opened, err: nil, configured: true}
		}

		// lefthook.yml is written before lefthook is installed, so a failed
		// install leaves a configuration behind.
		_, configured := existing(ctx)

		return hooksWritten{opened: opened, err: err, configured: configured}
	}
}

// hooksWritten reports how writing the configuration went, and whether a
// configuration exists after it, for the offer opened as opened.
type hooksWritten struct {
	opened     int
	err        error
	configured bool
}

var _ applier = hooksWritten{}

// refusal is what the offer takes as its write's answer: none once a
// configuration exists, since the offer is then done with whatever failed
// after it, or the reason none was written.
func (msg hooksWritten) refusal() error {
	if msg.configured {
		return nil
	}

	return msg.err
}

// apply closes the offer once a configuration exists, and makes it no more —
// saying why when lefthook then failed to install — or, while none does, keeps
// it open with the reason. An answer for an offer no longer open changes
// nothing.
func (msg hooksWritten) apply(m Model) (Model, tea.Cmd) {
	m, _, done := answered[hookgenOffer](m, msg.opened, msg.refusal())
	if !done {
		return m, nil
	}

	m.hookgen.read = m.hookgen.read.Told(nil)
	m = m.closeOverlay()

	if msg.err != nil {
		return m.noticedFailure(msg.err), nil
	}

	return m.noticed(m.marks.done + " wrote lefthook.yml and installed lefthook"), nil
}
