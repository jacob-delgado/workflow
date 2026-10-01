// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

// SlackTarget is a Slack user or user group, by its ID and the label it is
// shown by.
type SlackTarget struct {
	ID    string
	Label string
}

// OwnerLink is what was decided for one forge owner: whether they are on Slack,
// and if so, as whom — a user for a user owner, a user group for a team.
type OwnerLink struct {
	Owner   string
	OnSlack bool
	Slack   SlackTarget
}
