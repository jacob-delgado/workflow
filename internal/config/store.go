// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

// Store controls the on-disk store that keeps a little workflow state between
// sessions: the conveniences a session can see again — the commit scope last
// used in a repository, what was announced, the last issue list seen — and
// what the user decided — whom each code owner is on Slack, the groups a
// repository tags, the favorite directories. It is on by default and never
// holds a secret.
type Store struct {
	// Disabled turns the store off, so workflow keeps nothing between
	// sessions: it works the conveniences out afresh each time, tags no one in
	// an announcement, since it cannot keep who is whom, and keeps no
	// favorites. For a machine where no workflow state should touch the disk.
	Disabled bool `json:"disabled"`
}
