// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// errNoTerminal reports a question asked with nothing to answer it: stdin was
// closed, or piped and already read to its end.
var errNoTerminal = errors.New("no terminal to answer on")

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
	// StoreSecret saves secret in the OS keychain and returns the token_command
	// that reads it back. It is nil where storing is not wired for the platform,
	// and the guided flow then keeps the token in the file.
	StoreSecret func(secret string) (string, error)
	// Input is standard input, read to its end for a comment's text, as `git
	// commit -F -` reads a message, so a script never quotes it. It shares Line's
	// reader, so nothing read for one is lost to the other. Nil reads as empty.
	Input io.Reader
	// IsTerminal reports whether a stream is a terminal, so a slow command keeps
	// a progress note on stderr only where a person is watching it, never in a
	// pipe or a log. Nil reads every stream as no terminal.
	IsTerminal func(stream io.Writer) bool
}

// confirm asks a yes/no question, defaulting to no, so a bare enter is the safe
// answer. With nothing to read an answer from, it says so rather than passing
// on a bare end-of-file.
func confirm(prompt Prompt, question string) (bool, error) {
	answer, err := prompt.Line(question + " [y/N]: ")
	if errors.Is(err, io.EOF) {
		return false, errNoTerminal
	}

	if err != nil {
		return false, err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	return answer == "y" || answer == "yes", nil
}

// LineReader is Prompt.Line over input: it prints each prompt to prompts and
// reads one line from input, without its line ending. A last line typed with
// no newline after it is an answer, not the end of the input; the read after
// it is.
func LineReader(input *bufio.Reader, prompts io.Writer) func(prompt string) (string, error) {
	return func(prompt string) (string, error) {
		fmt.Fprint(prompts, prompt)

		line, err := input.ReadString('\n')
		if errors.Is(err, io.EOF) && line != "" {
			err = nil
		}

		return strings.TrimRight(line, "\r\n"), err
	}
}

// SecretReader is Prompt.Secret over input: it prints each prompt to prompts
// and reads one line from input without echoing it. Input that is no terminal
// — a pipe, a file — has no echo to turn off, and Line's buffered reader may
// already hold what follows its line, so it is answered as the end of the
// input, which every question words as how to go on without one.
func SecretReader(input *os.File, prompts io.Writer) func(prompt string) (string, error) {
	return func(prompt string) (string, error) {
		descriptor := int(input.Fd())
		if !term.IsTerminal(descriptor) {
			return "", io.EOF
		}

		fmt.Fprint(prompts, prompt)

		secret, err := term.ReadPassword(descriptor)

		fmt.Fprintln(prompts)

		if err != nil {
			return "", fmt.Errorf("reading the answer: %w", err)
		}

		return string(secret), nil
	}
}

// eraseLine returns the cursor to the start of the line and clears it, so a
// progress note replaces the one before it in place.
const eraseLine = "\r\x1b[K"

// progressNote is the one line a slow command keeps on stderr while it reads
// — "Reading Jira…" — when stderr is a terminal: each note replaces the last in
// place, and the line is erased before anything else is printed and when the
// command ends. Off a terminal it writes nothing, so a pipe or a log never
// holds one. It is a static line, with no spinner and nothing running beside
// the command.
type progressNote struct {
	// terminal is stderr when it is a terminal, and nil otherwise.
	terminal io.Writer
	showing  bool
}

// newProgressNote is a progress note on stderr, kept only when isTerminal says
// stderr is one.
func newProgressNote(stderr io.Writer, isTerminal func(io.Writer) bool) *progressNote {
	if isTerminal == nil || !isTerminal(stderr) {
		return &progressNote{}
	}

	return &progressNote{terminal: stderr}
}

// show replaces the note with "doing what…".
func (n *progressNote) show(doing, what string) {
	if n.terminal == nil {
		return
	}

	fmt.Fprint(n.terminal, eraseLine+doing+" "+sanitize.Line(what)+"…")
	n.showing = true
}

// clear erases the note, if one is showing.
func (n *progressNote) clear() {
	if !n.showing {
		return
	}

	fmt.Fprint(n.terminal, eraseLine)
	n.showing = false
}

// around is out with the note erased before anything is written to either
// stream, so what the command prints never lands on the note's line.
func (n *progressNote) around(out output) output {
	return output{artifact: noteErasing{note: n, out: out.artifact}, notes: noteErasing{note: n, out: out.notes}}
}

// noteErasing is a stream that erases a progress note before each write.
type noteErasing struct {
	note *progressNote
	out  io.Writer
}

var _ io.Writer = noteErasing{}

// Write erases the note, then writes p.
func (w noteErasing) Write(p []byte) (int, error) {
	w.note.clear()

	return w.out.Write(p) //nolint:wrapcheck // a stream's own error, passed through as the stream gave it
}
