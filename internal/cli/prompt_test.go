// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
)

func TestLineReaderKeepsAFinalLineTypedWithoutANewline(t *testing.T) {
	// Arrange
	read := cli.LineReader(bufio.NewReader(strings.NewReader("first\r\nlast")), io.Discard)

	// Act
	first, firstErr := read("? ")
	last, lastErr := read("? ")
	_, endErr := read("? ")

	// Assert
	if first != "first" || firstErr != nil || last != "last" || lastErr != nil {
		t.Errorf("reads = (%q, %v), (%q, %v); want \"first\" and \"last\", neither failing",
			first, firstErr, last, lastErr)
	}

	if !errors.Is(endErr, io.EOF) {
		t.Errorf("a read past the end = %v, want io.EOF", endErr)
	}
}

func TestLineReaderPrintsThePromptBeforeReading(t *testing.T) {
	// Arrange
	var prompts strings.Builder

	read := cli.LineReader(bufio.NewReader(strings.NewReader("answer\n")), &prompts)

	// Act
	_, err := read("Name: ")

	// Assert
	if err != nil || prompts.String() != "Name: " {
		t.Errorf("read = %v, printed %q; want the prompt printed and the answer read", err, prompts.String())
	}
}

// pipedInput is standard input as a pipe holding text, closed after it, as
// `printf text | workflow …` gives it.
func pipedInput(t *testing.T, text string) *os.File {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("making a pipe: %v", err)
	}

	t.Cleanup(func() { _ = reader.Close() })

	_, err = writer.WriteString(text)
	if err != nil {
		t.Fatalf("writing to the pipe: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("closing the pipe: %v", err)
	}

	return reader
}

func TestSecretReaderAnswersAPipeAsTheEndOfTheInput(t *testing.T) {
	// Arrange
	read := cli.SecretReader(pipedInput(t, "not-a-secret\n"), io.Discard)

	// Act
	_, err := read("Token: ")

	// Assert
	if !errors.Is(err, io.EOF) {
		t.Errorf("a secret read from a pipe = %v, want io.EOF, since a pipe cannot hide what it echoes", err)
	}
}

// pipedPrompt is the prompt the terminal's main wires, over stdin piped with
// text.
func pipedPrompt(t *testing.T, text string) cli.Prompt {
	t.Helper()

	stdin := pipedInput(t, text)
	reader := bufio.NewReader(stdin)

	return cli.Prompt{Line: cli.LineReader(reader, io.Discard), Secret: cli.SecretReader(stdin, io.Discard), Input: reader}
}

func TestGuidedInitOverAPipeSaysToWriteTheTemplate(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	_, err := runGuided(t, dir, pipedPrompt(t, "https://jira.example\n"), "config", "init")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "--template") || cli.ExitStatus(err) != 2 {
		t.Errorf("config init over a pipe = %v (exit %d), want the --template guidance and exit 2",
			err, cli.ExitStatus(err))
	}
}

func TestSlackLoginOverAPipeSaysToRunItAtATerminal(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	path := writeFile(t, dir, slackUserTokenFile)

	// Act
	_, err := runGuided(t, dir, pipedPrompt(t, slackClientID+"\n"), "slack", "login")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "at a terminal") || cli.ExitStatus(err) != 2 {
		t.Errorf("slack login over a pipe = %v (exit %d), want it sent to a terminal and exit 2",
			err, cli.ExitStatus(err))
	}

	unchanged(t, path, slackUserTokenFile)
}
