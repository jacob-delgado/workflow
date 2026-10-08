// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// wikiCasesFile is how Markdown converts, the cases the web's Preview answers
// to as well.
const wikiCasesFile = "testdata/wiki_from_markdown.json"

type wikiCases struct {
	About string `json:"about"`
	Cases []struct {
		Name     string `json:"name"`
		Markdown string `json:"markdown"`
		Want     string `json:"want"`
	} `json:"cases"`
}

// readWikiCases is the shared cases, refusing a field this side would ignore
// so a case cannot pin one copy and pass the other unread.
func readWikiCases(t *testing.T) wikiCases {
	t.Helper()

	file, err := os.Open(wikiCasesFile)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var read wikiCases

	err = decoder.Decode(&read)
	if err != nil {
		t.Fatalf("%s: %v", wikiCasesFile, err)
	}

	return read
}

func TestWikiFromMarkdownConvertsAsTheSharedCasesSay(t *testing.T) {
	t.Parallel()

	for _, testCase := range readWikiCases(t).Cases {
		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := jira.WikiFromMarkdown(testCase.Markdown)

			// Assert
			if got != testCase.Want {
				t.Errorf("WikiFromMarkdown(%q) =\n%q\nwant\n%q", testCase.Markdown, got, testCase.Want)
			}
		})
	}
}
