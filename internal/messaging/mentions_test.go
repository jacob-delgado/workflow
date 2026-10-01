// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/messaging"
)

// The user and user group these tests tag.
const (
	tagUser  = "U0M"
	tagGroup = "S0M"
)

func TestMentionsLineTagsEveryUserThenEveryGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	mentions, err := messaging.NewMentions([]string{tagUser, "W0M"}, []string{tagGroup})
	if err != nil {
		t.Fatalf("NewMentions = %v, want valid IDs accepted", err)
	}

	// Act
	line := mentions.Line()

	// Assert
	if want := "cc <@" + tagUser + "> <@W0M> <!subteam^" + tagGroup + ">"; line != want {
		t.Errorf("Line = %q, want %q", line, want)
	}
}

func TestMentionsLineIsEmptyWithNobodyToTag(t *testing.T) {
	t.Parallel()

	// Arrange
	mentions := messaging.Mentions{Users: []messaging.SlackUserID{{}}, Groups: nil}

	// Act
	line := mentions.Line()

	// Assert
	if line != "" {
		t.Errorf("Line = %q, want nothing when no valid ID is held", line)
	}
}

func TestMentionsLineTagsEachIDOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	mentions, err := messaging.NewMentions([]string{tagUser, tagUser}, []string{tagGroup, tagGroup})
	if err != nil {
		t.Fatalf("NewMentions = %v, want valid IDs accepted", err)
	}

	// Act
	line := mentions.Line()

	// Assert
	if want := "cc <@" + tagUser + "> <!subteam^" + tagGroup + ">"; line != want {
		t.Errorf("Line = %q, want %q", line, want)
	}
}

func TestNewMentionsRefusesAnythingButAnID(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ users, groups []string }{
		"a user that is markup":  {users: []string{"U0M> <!channel"}},
		"a group that is a user": {groups: []string{tagUser}},
		"a user that is a group": {users: []string{"S01"}},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := messaging.NewMentions(test.users, test.groups)

			// Assert
			if !errors.Is(err, messaging.ErrInvalidSlackID) {
				t.Errorf("NewMentions = %v, want ErrInvalidSlackID", err)
			}
		})
	}
}

func FuzzMentions(f *testing.F) {
	f.Add("U01,W02", "S03")
	f.Add("<!channel>,U01> <!here", "S0&lt;,!subteam^S1")
	f.Add("", "")
	f.Add("UAB\n,U\u00c9AB", "S12 ")

	f.Fuzz(func(t *testing.T, users, groups string) {
		// Arrange
		mentions := parsedMentions(users, groups)

		// Act
		line := mentions.Line()

		// Assert
		if line == "" && len(mentions.Users)+len(mentions.Groups) > 0 {
			t.Errorf("Line is empty, want a tag for each of %v", mentions)
		}

		if line != "" && !onlyTags(line) {
			t.Errorf("Line = %q, want \"cc\" then only user and group tokens", line)
		}
	})
}

// parsedMentions tags each comma-separated value of users and groups that
// parses as its kind.
func parsedMentions(users, groups string) messaging.Mentions {
	var mentions messaging.Mentions

	for raw := range strings.SplitSeq(users, ",") {
		user, err := messaging.ParseSlackUser(raw)
		if err == nil {
			mentions.Users = append(mentions.Users, user)
		}
	}

	for raw := range strings.SplitSeq(groups, ",") {
		group, err := messaging.ParseSlackGroup(raw)
		if err == nil {
			mentions.Groups = append(mentions.Groups, group)
		}
	}

	return mentions
}

// onlyTags reports whether line is "cc" then one or more space-separated user
// or group tokens, and nothing else.
func onlyTags(line string) bool {
	return regexp.MustCompile(`^cc( <@[UW][A-Z0-9]{2,}>| <!subteam\^S[A-Z0-9]{2,}>)+$`).MatchString(line)
}
