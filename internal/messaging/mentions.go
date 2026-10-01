// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging

import (
	"slices"
	"strings"
)

// Mentions is who an announcement tags: users and user groups, by validated
// ID alone. Its IDs come only from ParseSlackUser and ParseSlackGroup, which is
// what lets Line place them in markup without escaping.
type Mentions struct {
	Users  []SlackUserID
	Groups []SlackGroupID
}

// NewMentions validates every user and group ID, refusing the lot with
// ErrInvalidSlackID when any one is not shaped like its kind.
func NewMentions(users, groups []string) (Mentions, error) {
	mentions := Mentions{Users: make([]SlackUserID, 0, len(users)), Groups: make([]SlackGroupID, 0, len(groups))}

	for _, raw := range users {
		user, err := ParseSlackUser(raw)
		if err != nil {
			return Mentions{}, err
		}

		mentions.Users = append(mentions.Users, user)
	}

	for _, raw := range groups {
		group, err := ParseSlackGroup(raw)
		if err != nil {
			return Mentions{}, err
		}

		mentions.Groups = append(mentions.Groups, group)
	}

	return mentions, nil
}

// Line is the Slack line that tags everyone once — "cc <@U…> <!subteam^S…>" —
// or nothing when there is no one to tag. It carries no labels: Slack shows
// each tag by its current name, and a label is anyone's to write, so it has no
// place inside the markup. It is appended after the escaped announcement text,
// never passed through slackEscape, which would break the tags.
func (m Mentions) Line() string {
	tokens := make([]string, 0, len(m.Users)+len(m.Groups))

	for _, user := range m.Users {
		tokens = appendOnce(tokens, user.id, "<@"+user.id+">")
	}

	for _, group := range m.Groups {
		tokens = appendOnce(tokens, group.id, "<!subteam^"+group.id+">")
	}

	if len(tokens) == 0 {
		return ""
	}

	return "cc " + strings.Join(tokens, " ")
}

// appendOnce adds token unless id is the zero ID or token is already there.
func appendOnce(tokens []string, id, token string) []string {
	if id == "" || slices.Contains(tokens, token) {
		return tokens
	}

	return append(tokens, token)
}
