// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bufio"
	"errors"
	"io"
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
