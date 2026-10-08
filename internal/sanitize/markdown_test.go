// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package sanitize_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// markdownMarkup is every character Markdown reads as markup inside a line.
const markdownMarkup = "\\`[]()*_~|#>"

func TestEscapeMarkdownEscapesEveryCharacterMarkdownReads(t *testing.T) {
	t.Parallel()

	for _, character := range markdownMarkup {
		t.Run(string(character), func(t *testing.T) {
			t.Parallel()

			// Act
			escaped := sanitize.EscapeMarkdown("a" + string(character) + "b")

			// Assert
			if want := `a\` + string(character) + "b"; escaped != want {
				t.Errorf("EscapeMarkdown = %q, want %q", escaped, want)
			}
		})
	}
}

func TestUnescapeMarkdownUndoesEscapeMarkdownForEveryCharacter(t *testing.T) {
	t.Parallel()

	for _, character := range markdownMarkup + "plain" {
		t.Run(string(character), func(t *testing.T) {
			t.Parallel()

			// Arrange
			text := "a" + string(character) + string(character) + "b"

			// Act
			got := sanitize.UnescapeMarkdown(sanitize.EscapeMarkdown(text))

			// Assert
			if got != text {
				t.Errorf("UnescapeMarkdown(EscapeMarkdown(%q)) = %q, want it back", text, got)
			}
		})
	}
}

func FuzzUnescapeMarkdownUndoesEscapeMarkdown(f *testing.F) {
	for _, seed := range []string{"", "plain", `\`, `\\*`, "[a](b)", "# heading", "`code` > quote", `a\b`} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, text string) {
		// Act
		got := sanitize.UnescapeMarkdown(sanitize.EscapeMarkdown(text))

		// Assert
		if got != text {
			t.Errorf("UnescapeMarkdown(EscapeMarkdown(%q)) = %q, want it back", text, got)
		}
	})
}
