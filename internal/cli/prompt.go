// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import "strings"

// Prompt is the guided command's seams: reading an answer, and offering to keep
// a secret in the operating system's keychain. Each is a seam so a test can
// drive the conversation without a terminal or a real keychain; the terminal
// and keychain implementations live in the main package, where no test needs
// either.
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
	// Compose opens draft in the editor and returns what was left, for a standup
	// note to be edited before it is posted.
	Compose func(draft string) (string, error)
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
