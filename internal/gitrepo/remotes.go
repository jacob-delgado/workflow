// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"slices"
	"strings"
)

// DefaultRemote is the remote a clone names for where it came from, and the
// answer to each remote question here that git's own configuration leaves
// open.
const DefaultRemote = "origin"

// upstreamRemote is the name a fork's clone conventionally gives the
// repository it was forked from, and ghBase the value gh writes to
// remote.<name>.gh-resolved on the remote it chose as the base.
const (
	upstreamRemote = "upstream"
	ghBase         = "base"
)

// remoteBranch is a branch on remote, "origin/main".
func remoteBranch(remote, name string) string {
	return remote + "/" + name
}

// BaseRemote is the remote pull requests merge into, as git's own
// configuration says: the one gh marked as the base (remote.<name>.gh-resolved
// set to base), else a remote named upstream — a fork's clone — else
// DefaultRemote. A configuration git cannot read answers DefaultRemote, as a
// repository with no remote does.
func (r Repository) BaseRemote(ctx context.Context) string {
	return parseRemotes(optional(ctx, r.run, "-C", r.dir, "config", "-z", "--get-regexp", `^remote\.`)).base()
}

// remotes is what the repository's configuration says of its remotes: their
// names, and the one gh marked as the base, if any.
type remotes struct {
	names  []string
	ghBase string
}

// parseRemotes reads git config -z --get-regexp: records of a key, a newline
// and a value, each ended by a NUL. A key is remote.<name>.<variable>, and a
// name may itself hold dots, so it is cut at the last one.
func parseRemotes(config string) remotes {
	var found remotes

	for record := range strings.SplitSeq(config, "\x00") {
		key, value, _ := strings.Cut(record, "\n")

		dot := strings.LastIndex(key, ".")
		if !strings.HasPrefix(key, "remote.") || dot <= len("remote.") {
			continue
		}

		name, variable := key[len("remote."):dot], key[dot+1:]

		switch {
		case variable == "url":
			found.names = append(found.names, name)
		case variable == "gh-resolved" && value == ghBase:
			found.ghBase = name
		}
	}

	return found
}

// base picks the base remote from what the configuration says: gh's base when
// it names a remote that exists, else upstream when it exists, else
// DefaultRemote.
func (found remotes) base() string {
	if found.ghBase != "" && slices.Contains(found.names, found.ghBase) {
		return found.ghBase
	}

	if slices.Contains(found.names, upstreamRemote) {
		return upstreamRemote
	}

	return DefaultRemote
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
