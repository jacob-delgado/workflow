// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
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
	// Secret prints prompt and reads one line without echoing it, for a token.
	Secret func(prompt string) (string, error)
	// StoreSecret saves secret in the OS keychain and returns the token_command
	// that reads it back. It is nil where storing is not wired for the platform,
	// and the guided flow then keeps the token in the file.
	StoreSecret func(secret string) (string, error)
	// Input is standard input, read to its end for a comment's text, as `git
	// commit -F -` reads a message, so a script never quotes it. It shares Line's
	// reader, so nothing read for one is lost to the other. Nil reads as empty.
	Input io.Reader
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
