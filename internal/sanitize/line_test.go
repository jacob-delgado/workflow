// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package sanitize_test

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// byteOrderMark is U+FEFF, which Go will not take written out in its source,
// even as an escape.
var byteOrderMark = string(rune(0xfeff)) //nolint:gochecknoglobals // a constant Go cannot spell as one

func TestLineKeepsANameOnOneLineWithNothingUnseen(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text string
		want string
	}{
		"an ordinary name":       {text: "internal/config/redact.go", want: "internal/config/redact.go"},
		"letters beyond ASCII":   {text: "docs/résumé-λόγος.md", want: "docs/résumé-λόγος.md"},
		"an emoji":               {text: "notes-\U0001F680.txt", want: "notes-\U0001F680.txt"},
		"a newline":              {text: "a\nb.go", want: "a" + replacement + "b.go"},
		"a tab":                  {text: "a\tb.go", want: "a" + replacement + "b.go"},
		"a line separator":       {text: "a\u2028b", want: "a" + replacement + "b"},
		"a paragraph separator":  {text: "a\u2029b", want: "a" + replacement + "b"},
		"a zero-width space":     {text: "ma\u200bin", want: "ma" + replacement + "in"},
		"a zero-width joiner":    {text: "ma\u200din", want: "ma" + replacement + "in"},
		"a word joiner":          {text: "ma\u2060in", want: "ma" + replacement + "in"},
		"a byte order mark":      {text: byteOrderMark + "main", want: replacement + "main"},
		"a soft hyphen":          {text: "ma\u00adin", want: "ma" + replacement + "in"},
		"a left-to-right mark":   {text: "ma\u200ein", want: "ma" + replacement + "in"},
		"a right-to-left mark":   {text: "ma\u200fin", want: "ma" + replacement + "in"},
		"an Arabic letter mark":  {text: "ma\u061cin", want: "ma" + replacement + "in"},
		"a tag character":        {text: "ma\U000E0041in", want: "ma" + replacement + "in"},
		"what Text already does": {text: "a\x1b]0;owned\x07b\xe2\x80\xaec", want: "ab" + replacement + "c"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := sanitize.Line(tt.text); got != tt.want {
				t.Errorf("Line(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestTextKeepsWhatProseIsWrittenWith(t *testing.T) {
	t.Parallel()

	// A joined emoji, a flag written with tag characters, and a word whose
	// letters a zero-width non-joiner keeps apart are how people write. They
	// are taken out of a name, where they can only mislead, and left in prose.
	cases := map[string]string{
		"a joined emoji":           "\U0001F468\u200d\U0001F469\u200d\U0001F467",
		"a flag of tag characters": "\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F",
		"a zero-width non-joiner":  "\u0645\u06cc\u200c\u062e\u0648\u0627\u0647\u0645",
		"a right-to-left mark":     "abc\u200f",
	}

	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := sanitize.Text(text); got != text {
				t.Errorf("Text(%q) = %q, want it unchanged", text, got)
			}
		})
	}
}

func FuzzLineIsOneLineWithNothingUnseen(f *testing.F) {
	f.Add("internal/config/redact.go")
	f.Add("a\nb\tc\u2028d")
	f.Add("ma\u200bi\u200en\U000E0041")
	f.Add("\x1b]0;owned\x07\xff\xc2\x9b")

	f.Fuzz(func(t *testing.T, text string) {
		// Act
		clean := sanitize.Line(text)

		// Assert
		if !utf8.ValidString(clean) || strings.ContainsAny(clean, "\n\t\r") {
			t.Fatalf("Line(%q) = %q, want valid UTF-8 on one line", text, clean)
		}

		for _, character := range clean {
			if forbidden(character) || unicode.In(character, unicode.Cf, unicode.Zl, unicode.Zp) {
				t.Fatalf("Line(%q) still carries %U", text, character)
			}
		}

		if again := sanitize.Line(clean); again != clean {
			t.Fatalf("Line(%q) = %q, and cleaning that again gives %q", text, clean, again)
		}
	})
}
