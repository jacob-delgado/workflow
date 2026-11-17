// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

// Tracking is what a branch's upstream is to it: none, another branch — a
// branch created from origin/main without --no-track tracks origin/main — or
// the branch a push of it lands on.
type Tracking int

// The upstreams a branch can have.
const (
	TracksNothing Tracking = iota
	TracksOther
	TracksOwn
)

// Counted is how git counted a branch against its upstream: not at all, for a
// branch with none; unknown, for an upstream git could not count it against;
// level with it; or ahead of it. It is independent of Tracking: a branch can
// track origin/main and be two commits ahead of it.
type Counted int

// The counts a branch can have against its upstream.
const (
	CountNone Counted = iota
	CountUnknown
	CountLevel
	CountAhead
)

// Tracking is what the branch's upstream is to it.
func (b Branch) Tracking() Tracking {
	switch b.Upstream {
	case "":
		return TracksNothing
	case b.PushTarget():
		return TracksOwn
	default:
		return TracksOther
	}
}

// Counted is how git counted the branch against its upstream: a count it could
// not give is not taken for none.
func (b Branch) Counted() Counted {
	switch {
	case b.Upstream == "":
		return CountNone
	case b.Uncounted:
		return CountUnknown
	case b.Ahead == 0:
		return CountLevel
	default:
		return CountAhead
	}
}

// PushTarget is the remote branch a push of this one lands on, "fork/feat" for
// feat pushed to fork.
func (b Branch) PushTarget() string {
	return b.PushRemote + "/" + b.Name
}

// Pushed reports whether the remote a push goes to holds this branch, under its
// own name, with every commit. The name matters: a branch created from
// origin/main without --no-track tracks origin/main, is ahead of it by nothing,
// and is not on the remote at all.
func (b Branch) Pushed() bool {
	return b.Name != "" && b.Tracking() == TracksOwn && b.Counted() == CountLevel
}

// Unpushed are the commits on this branch that have not reached the upstream,
// oldest first, so amending or fixing one up rewrites only local history. With
// no upstream every commit is unpushed; otherwise it is the last Ahead of them,
// none for a count git could not give.
//
// A branch long enough to be truncated returns none: Commits then keeps the
// oldest, while Ahead is counted against the full range, so the last Ahead of
// the retained commits are the wrong ones — some already pushed. Rather than
// risk offering pushed history to rewrite, a truncated branch offers nothing.
func (b Branch) Unpushed() []Commit {
	if b.Truncated {
		return nil
	}

	if b.Counted() == CountNone {
		return b.Commits
	}

	return b.Commits[max(0, len(b.Commits)-b.Ahead):]
}

// UpstreamHoldsAll reports an upstream that holds every commit on this branch:
// it has one and git counted it no commits ahead of it. It is one of the two
// things that make a force delete safe. With no upstream — never pushed under
// its own name, or one the forge deleted after a merge and a prune then
// dropped — nothing is known to hold the commits, so it reports false.
func (b Branch) UpstreamHoldsAll() bool {
	return b.Counted() == CountLevel
}
