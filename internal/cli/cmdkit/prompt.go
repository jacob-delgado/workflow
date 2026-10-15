// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/console"
	"github.com/jacob-delgado/workflow/internal/config"
)

// ErrNoTerminal reports a question asked with nothing to answer it: stdin was
// closed, or piped and already read to its end.
var ErrNoTerminal = errors.New("no terminal to answer on")

// Prompt is how a command asks the person at the terminal: reading an answer,
// for the guided init and every scriptable write's yes/no question; reading
// standard input whole, for a comment's text; keeping a
// secret in the operating system's keychain; and composing a note in the
// editor. Each is a seam so a test can drive the conversation without a
// terminal, a real keychain or an editor. The main package wires the two
// terminal reads, which no test can make, and takes the keychain and the editor
// from their own packages.
//
// A zero Prompt is enough for a command that never asks anything — the doctor,
// or the reference generator that only walks the tree.
type Prompt struct {
	// Line prints prompt and reads one visible line, for an address or a choice.
	Line func(prompt string) (string, error)
	// Secret prints prompt and reads one line without echoing it, for a token;
	// with no terminal to read it from, it answers io.EOF.
	Secret func(prompt string) (string, error)
	// StoreSecret saves secret in the OS keychain under the item service names,
	// under ctx. It is nil where storing is not wired for the platform, and the
	// guided flow then keeps the token in the file.
	StoreSecret func(ctx context.Context, service, secret string) error
	// Input is standard input, read to its end for a comment's text, as `git
	// commit -F -` reads a message, so a script never quotes it. It shares Line's
	// reader, so nothing read for one is lost to the other. Nil reads as empty.
	Input io.Reader
	// IsTerminal reports whether a stream is a terminal, so a slow command keeps
	// a live line on stderr only where a person is watching it, never in a
	// pipe or a log, and prints its artifact beside the line only where stdout
	// is no terminal. Nil reads every stream as no terminal.
	IsTerminal func(stream io.Writer) bool
}

// Confirm asks a yes/no question, defaulting to no, so a bare enter is the safe
// answer. With nothing to read an answer from, it says so rather than passing
// on a bare end-of-file.
func Confirm(prompt Prompt, question string) (bool, error) {
	answer, err := prompt.Line(question + " [y/N]: ")
	if errors.Is(err, io.EOF) {
		return false, ErrNoTerminal
	}

	if err != nil {
		return false, err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	return answer == "y" || answer == "yes", nil
}

// Waiting runs wait, a slow command's reads, keeping its live line on stderr,
// drawn as look asks, while it does when stderr is a terminal a person
// watches, and keeping none in a pipe or a log, where only what it says
// prints. Its notes are said above the line, and its artifact goes to stdout,
// unless stdout is the terminal too.
func Waiting(
	cmd *cobra.Command, prompt Prompt, look console.Look, wait func(out Output, live console.Wait) error,
) error {
	notes := NotesOf(cmd)
	if !watched(prompt, cmd.ErrOrStderr()) {
		return notes.Unwatched(func(live console.Wait) error { return wait(OutputOf(cmd), live) })
	}

	return notes.Watched(look, func(live console.Wait) error {
		return wait(Output{Artifact: artifactBeside(cmd, prompt, live), Notes: live.Notes()}, live)
	})
}

// LiveLook is how a live line is drawn under ui: in ASCII when ui.ascii asks,
// and its mark still when ui.motion reduces motion.
func LiveLook(ui config.UI) console.Look {
	return console.Look{ASCII: ui.ASCII, Still: ui.ReducedMotion()}
}

// artifactBeside is where cmd writes its artifact while live keeps its line:
// stdout, unless stdout is the terminal the line is on.
func artifactBeside(cmd *cobra.Command, prompt Prompt, live console.Wait) io.Writer {
	if !watched(prompt, TerminalOut(cmd)) {
		return cmd.OutOrStdout()
	}

	// Trade-off TRADE-43: with both streams on the terminal, the artifact
	// prints above the live line, on stderr, rather than under its feet.
	return live.Above()
}

// watched reports whether stream is a terminal a person watches, as prompt
// tells one: the stream itself, beneath the notes' Stream.
func watched(prompt Prompt, stream io.Writer) bool {
	return prompt.IsTerminal != nil && prompt.IsTerminal(console.Underlying(stream))
}
