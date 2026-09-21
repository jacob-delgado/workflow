// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// starBullet is the wiki bullet every Markdown bullet marker converts to; it is
// also the source form of the star-marked bullet, which must not be read as
// emphasis.
const starBullet = "* item"

func TestWikiFromMarkdown(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		in   string
		want string
	}{
		"plain prose is left alone": {
			in:   `Tokens reach the log when the header is empty. See "quotes".`,
			want: `Tokens reach the log when the header is empty. See "quotes".`,
		},
		"headings become h-levels": {
			in:   "# Title\n## Section",
			want: "h1. Title\nh2. Section",
		},
		"bold and italic": {
			in:   "This is **bold** and *italic* text.",
			want: "This is *bold* and _italic_ text.",
		},
		"bold italic together": {
			in:   "This is ***important*** text.",
			want: "This is *_important_* text.",
		},
		"spaced asterisks are not emphasis": {
			in:   "the array is 2 * 3 * 4 elements",
			want: "the array is 2 * 3 * 4 elements",
		},
		"a spaced asterisk beside a real italic": {
			in:   "a * b and *ital*",
			want: "a * b and _ital_",
		},
		"intraword double underscores are literal": {
			in:   "call a__b__c helper",
			want: "call a__b__c helper",
		},
		"the underscore forms": {
			in:   "A __bold__ and an _italic_ word.",
			want: "A *bold* and an _italic_ word.",
		},
		"inline code becomes braces": {
			in:   "Call `doThing()` first.",
			want: "Call {{doThing()}} first.",
		},
		"a code span shields its contents": {
			in:   "Literal `**stars**` but real **bold**.",
			want: "Literal {{**stars**}} but real *bold*.",
		},
		"links": {
			in:   "See [the docs](https://example.com/x) for more.",
			want: "See [the docs|https://example.com/x] for more.",
		},
		"a link URL is copied verbatim": {
			in:   "see [docs](https://ex.com/foo*bar*baz) now",
			want: "see [docs|https://ex.com/foo*bar*baz] now",
		},
		"a link URL may hold balanced parentheses": {
			in:   "[a](https://en.wikipedia.org/wiki/Foo_(bar))",
			want: "[a|https://en.wikipedia.org/wiki/Foo_(bar)]",
		},
		"an image becomes Jira image markup": {
			in:   "![diagram](https://x/i.png)",
			want: "!https://x/i.png!",
		},
		"an image beside a link": {
			in:   "![img](a.png) and [link](b)",
			want: "!a.png! and [link|b]",
		},
		"a bullet list": {
			in:   "- first\n- second",
			want: "* first\n* second",
		},
		"a numbered list": {
			in:   "1. one\n2. two",
			want: "# one\n# two",
		},
		"a blockquote": {
			in:   "> a remark",
			want: "bq. a remark",
		},
		"a blockquote with emphasis": {
			in:   "> a **bold** remark",
			want: "bq. a *bold* remark",
		},
		"a star bullet": {
			in:   starBullet,
			want: starBullet,
		},
		"a plus bullet": {
			in:   "+ item",
			want: starBullet,
		},
		"strikethrough": {
			in:   "~~gone~~ now",
			want: "-gone- now",
		},
		"a fenced code block is bracketed and left verbatim": {
			in:   "before\n```go\nx := **notBold**\n```\nafter",
			want: "before\n{code:go}\nx := **notBold**\n{code}\nafter",
		},
		"an indented fence is not read as a fence": {
			in:   "text\n    ```go\nmore **bold**",
			want: "text\n    ```go\nmore *bold*",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := jira.WikiFromMarkdown(testCase.in)

			// Assert
			if got != testCase.want {
				t.Errorf("WikiFromMarkdown(%q) =\n%q\nwant\n%q", testCase.in, got, testCase.want)
			}
		})
	}
}
