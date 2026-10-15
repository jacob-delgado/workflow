// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"io"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
)

// Scripted answers the guided prompts in order: visible lines from lines,
// secrets from secrets. A prompt past the end of its list reads as blank.
func Scripted(lines, secrets []string) cmdkit.Prompt {
	return cmdkit.Prompt{Line: answersFrom(&lines), Secret: answersFrom(&secrets)}
}

// Answering is a prompt that gives every question the same answer and keeps
// each question it was asked, for a test of what a command asks.
func Answering(answer string, asked *[]string) cmdkit.Prompt {
	reply := func(question string) (string, error) {
		*asked = append(*asked, question)

		return answer, nil
	}

	return cmdkit.Prompt{Line: reply, Secret: reply}
}

// answersFrom hands back each answer in turn, then blanks.
func answersFrom(answers *[]string) func(string) (string, error) {
	return func(string) (string, error) {
		if len(*answers) == 0 {
			return "", nil
		}

		next := (*answers)[0]
		*answers = (*answers)[1:]

		return next, nil
	}
}

// AnswersThenEnds answers each question with the next of answers, then reads
// the end of the input, as a pipe that runs dry does.
func AnswersThenEnds(answers ...string) func(string) (string, error) {
	return func(string) (string, error) {
		if len(answers) == 0 {
			return "", io.EOF
		}

		next := answers[0]
		answers = answers[1:]

		return next, nil
	}
}

// EraseBelow is what the live line's program sends as it ends, erasing the
// line it kept.
const EraseBelow = "\x1b[J"

// LiveMarks are the frames the live line's mark turns through.
func LiveMarks() []string {
	return []string{"◐", "◓", "◑", "◒"}
}

// ASCIIMarks are the frames the live line's mark turns through in ASCII.
func ASCIIMarks() []string {
	return []string{"|", "/", "-", `\`}
}

// DrawnWith reports whether shown draws want behind one of marks, the frame
// the live line's mark had turned to.
func DrawnWith(shown string, marks []string, want string) bool {
	for _, mark := range marks {
		if strings.Contains(shown, mark+" "+want) {
			return true
		}
	}

	return false
}

// MarkedLive reports whether shown holds the live line's mark.
func MarkedLive(shown string) bool {
	for _, mark := range LiveMarks() {
		if strings.Contains(shown, mark) {
			return true
		}
	}

	return false
}

// AtTerminal is UnusedPrompt with every stream flagged a terminal, as
// main flags a real one.
func AtTerminal(t *testing.T) cmdkit.Prompt {
	t.Helper()

	prompt := UnusedPrompt(t)
	prompt.IsTerminal = func(io.Writer) bool { return true }

	return prompt
}

// UnicodeLine is every character the live line draws by default and never in
// ASCII: its mark's frames, and its bar's.
func UnicodeLine() []string {
	return append(LiveMarks(), "━", "─")
}
