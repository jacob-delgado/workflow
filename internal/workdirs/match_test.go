// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package workdirs_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/workdirs"
)

// casesFile is the matcher's cases, which the web's copy of it answers to as
// well.
const casesFile = "../../testdata/twins/directory_match.json"

// matched is a name a case says matches: where its letters matched, and
// whether it starts with them.
type matched struct {
	Name    string `json:"name"`
	Letters []int  `json:"letters"`
	Prefix  bool   `json:"prefix"`
}

type matchCorpus struct {
	About string `json:"about"`
	Cases []struct {
		Name   string    `json:"name"`
		Listed []string  `json:"listed"`
		Typed  string    `json:"typed"`
		Want   []matched `json:"want"`
	} `json:"cases"`
}

// readMatchCases is the shared cases, refusing a field this side would ignore
// so a case cannot pin one copy and pass the other unread.
func readMatchCases(t *testing.T) matchCorpus {
	t.Helper()

	file, err := os.Open(casesFile)
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()

	var read matchCorpus

	err = decoder.Decode(&read)
	if err != nil {
		t.Fatalf("%s: %v", casesFile, err)
	}

	return read
}

// entries are directories by name, none of them a repository.
func entries(names ...string) []workdirs.Entry {
	listed := make([]workdirs.Entry, 0, len(names))
	for _, name := range names {
		listed = append(listed, workdirs.Entry{Name: name, Repository: false})
	}

	return listed
}

// matchesOf are what matched, as the shared cases write them.
func matchesOf(matches []workdirs.Match) []matched {
	written := make([]matched, 0, len(matches))
	for _, match := range matches {
		written = append(written, matched{Name: match.Entry.Name, Letters: match.Letters, Prefix: match.Prefix})
	}

	return written
}

// sameMatch reports one and other the same name, matched at the same letters.
func sameMatch(one, other matched) bool {
	return one.Name == other.Name && slices.Equal(one.Letters, other.Letters) && one.Prefix == other.Prefix
}

func TestMatchingFindsWhatTheSharedCasesName(t *testing.T) {
	t.Parallel()

	for _, tt := range readMatchCases(t).Cases {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			listed := entries(tt.Listed...)

			// Act
			got := matchesOf(workdirs.Matching(listed, tt.Typed))

			// Assert
			if !slices.EqualFunc(got, tt.Want, sameMatch) {
				t.Errorf("Matching(%q, %q) = %+v, want %+v", tt.Listed, tt.Typed, got, tt.Want)
			}
		})
	}
}
