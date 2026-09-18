// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import "context"

// RecentCommits reads the commits you made since a git date such as "yesterday"
// or "2 days ago", across the repository — for a standup. It filters to your own
// commits by the email git commits under, when one is set. Every subject is
// sanitized, because a commit is anyone's to write.
func (r Repository) RecentCommits(ctx context.Context, since string) ([]Commit, error) {
	args := []string{"-C", r.dir, "log", "-z", logFormat, "--all", "--since=" + since}

	if email := optional(ctx, r.run, "-C", r.dir, "config", "user.email"); email != "" {
		// git matches --author as a substring against name and email both; the
		// email is the precise identity a commit is made under.
		args = append(args, "--author="+email)
	}

	out, err := r.run(ctx, gitProgram, args...)
	if err != nil {
		return nil, readFailure(ctx, r.run, r.dir, "reading recent commits", err)
	}

	return parseLog(text(out)), nil
}
