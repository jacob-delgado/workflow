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
)

// LineReader is cmdkit.Prompt.Line over input: it prints each prompt to
// prompts and reads one line from input, without its line ending. A last line
// typed with no newline after it is an answer, not the end of the input; the
// read after it is.
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

// SecretReader is cmdkit.Prompt.Secret over input: it prints each prompt to
// prompts and reads one line from input without echoing it. Input that is no
// terminal — a pipe, a file — has no echo to turn off, and Line's buffered
// reader may already hold what follows its line, so it is answered as the end
// of the input, which every question words as how to go on without one.
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
