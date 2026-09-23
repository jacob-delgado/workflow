// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"errors"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/messaging"
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

// errorSentence rewrites a recognized error as a sentence in the interface's
// voice, reporting false for one it does not recognize. It is built here rather
// than as a package map because gochecknoglobals forbids the latter.
func errorSentence(err error) (string, bool) {
	for _, known := range []struct {
		sentinel error
		sentence string
	}{
		{jira.ErrNoCredential, "Jira is not set up. Add `jira.token` to `.workflow.json`."},
		{jira.ErrUnreachable, "Jira did not answer within 10 seconds. Check the VPN, then press `r`."},
		{forge.ErrNoToken, "No forge token found. Run `gh auth login`, or set `$GITHUB_TOKEN`."},
		{forge.ErrUnreachable, "The forge did not answer within 10 seconds. Check the network, then press `r`."},
		{messaging.ErrNoCredential, "Messaging is not set up. Add `messaging.webhook_url` to `.workflow.json`."},
		{messaging.ErrRejected, "Slack refused the post: check the bot is in the channel."},
	} {
		if errors.Is(err, known.sentinel) {
			return known.sentence, true
		}
	}

	return "", false
}

// failure draws an error the one way the interface says something broke. A
// recognized error reads as a sentence in the interface's voice, with the raw
// chain beneath it; an unrecognized one keeps its raw text. Either way the text
// is made safe here as well as where it was written.
func (m Model) failure(err error) string {
	glyph := m.marks.failed + " "

	sentence, known := errorSentence(err)
	if !known {
		return m.styles.failure.Render(glyph + sanitize.Text(err.Error()))
	}

	return m.styles.failure.Render(glyph+sentence) + "\n" +
		m.styles.label.Render(sanitize.Text(err.Error()))
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

// failureBlock is an error wrapped to a width and styled per row, so every row
// opens and closes its own color and none runs on into the border beside it.
func failureBlock(sty styles, marks glyphs, err error, width int) string {
	return sty.failure.Render(wrap(marks.failed+" "+sanitize.Text(err.Error()), width))
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

// failureWithin is failure for a pane: recognized as a sentence like failure,
// but wrapped to its width first and styled after.
func (m Model) failureWithin(err error, width int) string {
	sentence, known := errorSentence(err)
	if !known {
		return failureBlock(m.styles, m.marks, err, width)
	}

	return m.styles.failure.Render(wrap(m.marks.failed+" "+sentence, width)) + "\n" +
		m.styles.label.Render(wrap(sanitize.Text(err.Error()), width))
}
