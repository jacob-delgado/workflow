// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import "strings"

// Prompt reads a guided command's answers: a visible line, and a secret that is
// not echoed. Both are seams, so a test can script the conversation without a
// terminal; the terminal implementation, over golang.org/x/term, lives in the
// main package where no test needs a real TTY.
//
// A zero Prompt is enough for a command that never asks anything — the doctor,
// or the reference generator that only walks the tree.
type Prompt struct {
	// Line prints prompt and reads one visible line, for an address or a choice.
	Line func(prompt string) (string, error)
	// Secret prints prompt and reads one line without echoing it, for a token.
	Secret func(prompt string) (string, error)
}

// confirm asks a yes/no question, defaulting to no, so a bare enter is the safe
// answer.
func confirm(prompt Prompt, question string) (bool, error) {
	answer, err := prompt.Line(question + " [y/N]: ")
	if err != nil {
		return false, err
	}

	answer = strings.ToLower(strings.TrimSpace(answer))

	return answer == "y" || answer == "yes", nil
}
