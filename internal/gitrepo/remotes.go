// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import "context"

// DefaultRemote is the remote a clone names for where it came from, and the
// answer to each remote question here that git's own configuration leaves
// open.
const DefaultRemote = "origin"

// remoteBranch is a branch on DefaultRemote, "origin/main".
func remoteBranch(name string) string {
	return DefaultRemote + "/" + name
}

// PushRemote is the remote a push of branch goes to, as git itself decides
// it: the branch's own branch.<name>.pushRemote, then the repository's
// remote.pushDefault — a fork's clone pushes to its own remote — then the
// remote the branch tracks, branch.<name>.remote, then DefaultRemote. git
// exits non-zero for a key that is unset, which optional reads as the empty
// string, so each unset key falls through cleanly. A detached HEAD, with no
// branch to keep a key under, asks only for the default.
func (r Repository) PushRemote(ctx context.Context, branch string) string {
	keys := []string{"remote.pushDefault"}
	if branch != "" {
		keys = []string{"branch." + branch + ".pushRemote", "remote.pushDefault", "branch." + branch + ".remote"}
	}

	for _, key := range keys {
		if remote := optional(ctx, r.run, "-C", r.dir, "config", "--get", key); remote != "" {
			return remote
		}
	}

	return DefaultRemote
}
