// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package sanitize_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// forbidden reports a rune that must never reach the terminal from a server.
func forbidden(character rune) bool {
	switch {
	case character == '\n' || character == '\t':
		return false
	case character < 0x20, character >= 0x7f && character <= 0x9f:
		return true
	case character >= 0x202a && character <= 0x202e, character >= 0x2066 && character <= 0x2069:
		return true
	default:
		return false
	}
}

// eachString walks a decoded JSON value and yields every string in it, keys
// included: a key is as printable as a value.
func eachString(value any, yield func(string)) {
	switch typed := value.(type) {
	case string:
		yield(typed)
	case []any:
		for _, item := range typed {
			eachString(item, yield)
		}
	case map[string]any:
		for key, item := range typed {
			yield(key)
			eachString(item, yield)
		}
	}
}

func FuzzJSONKeepsValidJSONValidAndFreeOfControls(f *testing.F) {
	f.Add([]byte(`{"summary":"a\u001b[2Jb","list":["\u009b","\\u001b"]}`))
	f.Add([]byte("{\"k\u202e\":\"\xc2\x9b\\r\\n\"}"))
	f.Add([]byte(`["\\\u001b", "\ud83d\ude80", "\u00e9"]`))
	f.Add([]byte(`"\u00`))

	f.Fuzz(func(t *testing.T, raw []byte) {
		if !json.Valid(raw) {
			return
		}

		clean := sanitize.JSON(raw)
		if !json.Valid(clean) {
			t.Fatalf("valid JSON %q became invalid: %q", raw, clean)
		}

		// UseNumber, because valid JSON may hold a number no float64 can: the
		// fuzzer's 1e700 is a harness failure, not a sanitizer one.
		decoder := json.NewDecoder(bytes.NewReader(clean))
		decoder.UseNumber()

		var value any

		err := decoder.Decode(&value)
		if err != nil {
			t.Fatalf("sanitized JSON %q does not decode: %v", clean, err)
		}

		eachString(value, func(text string) {
			for _, character := range text {
				if forbidden(character) {
					t.Fatalf("%q decoded to %q, which still carries %U", raw, text, character)
				}
			}
		})
	})
}

func FuzzTextNeverCarriesAControl(f *testing.F) {
	f.Add("\x1b[1;31mFAIL\x1b[0m")
	f.Add("a\xc2\x9bb\xe2\x80\xae\r\n")
	f.Add("\x1b]8;;http://x\x1b\\link\x1b]8;;\x1b\\")
	f.Add("\xff\xfe\x1b")

	f.Fuzz(func(t *testing.T, text string) {
		for _, character := range sanitize.Text(text) {
			if forbidden(character) {
				t.Fatalf("Text(%q) still carries %U", text, character)
			}
		}
	})
}
