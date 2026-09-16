// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package sanitize_test

import (
	"encoding/json"
	"testing"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// replacement is what a neutralized character becomes: visible, one cell wide,
// and unmistakably not what the server sent.
const replacement = "\ufffd"

// decoded runs a JSON string literal through JSON and decodes what comes out.
func decoded(t *testing.T, literal string) string {
	t.Helper()

	var value string

	err := json.Unmarshal(sanitize.JSON([]byte(literal)), &value)
	if err != nil {
		t.Fatalf("the sanitized form of %s is not valid JSON: %v", literal, err)
	}

	return value
}

func TestJSONNeutralizesControlCharacters(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		literal string
		want    string
	}{
		// ESC begins every terminal control sequence: clearing the screen,
		// moving the cursor over what is drawn, writing the clipboard.
		"escape":                  {literal: `"a\u001b[2Jb"`, want: "a" + replacement + "[2Jb"},
		"escape in capitals":      {literal: `"\u001B]52;c;aGk=\u0007"`, want: replacement + "]52;c;aGk=" + replacement},
		"null":                    {literal: `"\u0000"`, want: replacement},
		"backspace and form feed": {literal: `"a\bb\fc"`, want: "a" + replacement + "b" + replacement + "c"},
		"delete":                  {literal: `"\u007f"`, want: replacement},
		// The 8-bit controls: U+009B is a whole CSI on its own to a terminal
		// reading C1.
		"escaped c1":           {literal: `"\u009b2J"`, want: replacement + "2J"},
		"raw c1":               {literal: "\"\xc2\x9b2J\"", want: replacement + "2J"},
		"raw delete":           {literal: "\"a\x7fb\"", want: "a" + replacement + "b"},
		"bidi override":        {literal: `"abc\u202edef"`, want: "abc" + replacement + "def"},
		"raw bidi isolate":     {literal: "\"abc\xe2\x81\xa6def\"", want: "abc" + replacement + "def"},
		"escaped bidi isolate": {literal: `"\u2069"`, want: replacement},
		// A carriage return moves the cursor back over what is already on the
		// line. Windows line endings in a description lose it and keep the
		// newline.
		"carriage return":         {literal: `"a\r\nb"`, want: "a\nb"},
		"escaped carriage return": {literal: `"a\u000d\u000ab"`, want: "a\nb"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := decoded(t, tt.literal); got != tt.want {
				t.Errorf("%s decoded to %q, want %q", tt.literal, got, tt.want)
			}
		})
	}
}

func TestJSONLeavesOrdinaryTextAlone(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		literal string
		want    string
	}{
		"newline and tab": {literal: `"a\nb\tc"`, want: "a\nb\tc"},
		"escaped newline": {literal: `"a\u000ab\u0009c"`, want: "a\nb\tc"},
		"accents":         {literal: "\"\\u00e9t\xc3\xa9 d\xc3\xa9j\xc3\xa0\"", want: "\xc3\xa9t\xc3\xa9 d\xc3\xa9j\xc3\xa0"},
		"quotes":          {literal: `"say \"hi\" \/ bye"`, want: `say "hi" / bye`},
		// A backslash the server escaped is text, not the start of an escape:
		// this is six printable characters, which are harmless.
		"escaped backslash": {literal: `"\\u001b"`, want: `\u001b`},
		// ...but an escaped backslash followed by a real escape is both.
		"backslash then escape": {literal: `"\\\u001b"`, want: `\` + replacement},
		"surrogate pair":        {literal: `"ship it \ud83d\ude80"`, want: "ship it \xf0\x9f\x9a\x80"},
		"right-to-left text":    {literal: "\"\xd7\xa9\xd7\x9c\xd7\x95\xd7\x9d\"", want: "\xd7\xa9\xd7\x9c\xd7\x95\xd7\x9d"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := decoded(t, tt.literal); got != tt.want {
				t.Errorf("%s decoded to %q, want %q", tt.literal, got, tt.want)
			}
		})
	}
}

func TestJSONLeavesMalformedInputForTheDecoderToReject(t *testing.T) {
	t.Parallel()

	for _, malformed := range []string{`"\u00`, `"\u00zz"`, `"\`} {
		if got := string(sanitize.JSON([]byte(malformed))); got != malformed {
			t.Errorf("JSON(%q) = %q, want it untouched", malformed, got)
		}
	}
}

func TestJSONReplacesInvalidUTF8SoNoControlCanBeAssembled(t *testing.T) {
	t.Parallel()

	// Found by FuzzJSONKeepsValidJSONValidAndFreeOfControls: each of 0xC2 and
	// 0x97 is invalid alone, and dropping the \r between them joined them into
	// U+0097, a C1 control.
	if got := decoded(t, "\"\xc2\\r\x97\""); got != replacement+replacement {
		t.Errorf("decoded to %q, want each invalid byte replaced", got)
	}
}

func TestTextStripsTerminalSequencesAndNeutralizesTheRest(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text string
		want string
	}{
		// A hook's colors are sequences, and removing them whole leaves the
		// text readable rather than littered with replacement characters.
		"colors":    {text: "\x1b[1;31mFAIL\x1b[0m lint", want: "FAIL lint"},
		"osc title": {text: "\x1b]0;owned\x07after", want: "after"},
		// An escape with nothing after it is an unfinished sequence, and goes.
		"bare escape":      {text: "a\x1b", want: "a"},
		"c1 control":       {text: "a\xc2\x9bb", want: "a" + replacement + "b"},
		"carriage return":  {text: "progress 10%\rprogress 100%", want: "progress 10%progress 100%"},
		"newline and tab":  {text: "a\n\tb", want: "a\n\tb"},
		"invalid utf-8":    {text: "a\xffb", want: "a" + replacement + "b"},
		"bidi override":    {text: "a\xe2\x80\xaeb", want: "a" + replacement + "b"},
		"ordinary unicode": {text: "passes \xe2\x9c\x94 (0.01 seconds)", want: "passes \xe2\x9c\x94 (0.01 seconds)"},
		"box drawing":      {text: "\xe2\x94\x83  lint", want: "\xe2\x94\x83  lint"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := sanitize.Text(tt.text); got != tt.want {
				t.Errorf("Text(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}
