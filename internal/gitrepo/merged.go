// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// ErrNotFetched is a commit a pull request merged at that this clone does not
// hold and could not fetch, so what was made on the branch after it cannot be
// counted.
var ErrNotFetched = errors.New("is not in this clone, and could not be fetched")

// FetchRefTimeout bounds the fetch of the one ref a pull request's head is
// kept at: a few commits at most, so a remote that has not sent them in a
// minute is not waited on for as long as a finish would stand still.
//
// Trade-off TRADE-68: nothing is kept of a fetch that failed, so every look at
// the merged pull request tries again, and waits up to this bound.
const FetchRefTimeout = time.Minute

// FetchRefCommand fetches ref from remote into this clone's objects, and
// writes no ref and no FETCH_HEAD: the fetch of a merged pull request's head
// that a finish makes when this clone lacks the commit it merged at, which
// moves nothing a developer reads.
//
// Prompts are off, as they are for every fetch: nobody can answer for a
// credential from inside the interface. "--" ends git's options, so the
// remote's name is never read as one.
func FetchRefCommand(dir, remote, ref string) proc.Command {
	return proc.Command{
		Dir:  dir,
		Name: gitProgram,
		Args: []string{"fetch", "--no-tags", "--no-write-fetch-head", "--", remote, ref},
		Env:  []string{noTerminalPrompt},
	}
}

// NamedAfterMerge is how many of the commits made after a merge MadeAfter
// lists: a refusal names a few and counts the rest, so a branch reused months
// later is not read in full on every look.
const NamedAfterMerge = 5

// ErrNotACount is git answering a count with something other than a number.
var ErrNotACount = errors.New("git's count is not a number")

// AfterMerge is what a branch holds past the commit its pull request merged
// at: made after the merge, so no pull request carried them.
type AfterMerge struct {
	// Latest is the newest of the commits Base does not hold, newest first, at
	// most NamedAfterMerge of them.
	Latest []Commit
	// Own counts the commits past the merge that Base does not hold, and
	// OnBase those it does: after a squash merge, a branch moved onto the
	// newer base holds the squash and every commit the base took since.
	Own, OnBase int
	// Base is the base remote's branch, as remote/branch, that told them
	// apart: empty where this clone holds none, every commit counted as Own.
	Base string
}

// Any reports commits made after the merge, on the base or not.
func (a AfterMerge) Any() bool {
	return a.Own+a.OnBase > 0
}

// MadeAfter is what HEAD holds past merged, the commit a pull request of this
// branch merged at, told apart by base, the base remote's branch as
// remote/branch, empty for none. None means HEAD is merged or an ancestor of
// it. A merged commit this clone does not hold — pushed from elsewhere, and
// never fetched here — is fetched first through fetch, and a fetch that
// fails, or that does not bring it, answers ErrNotFetched. It knows of none
// where merged is not a commit's hash: the forge named none.
func (r Repository) MadeAfter(
	ctx context.Context, merged, base string, fetch func(context.Context) error,
) (AfterMerge, error) {
	if !isHex(merged) {
		return AfterMerge{}, nil
	}

	err := r.holdOrFetch(ctx, merged, fetch)
	if err != nil {
		return AfterMerge{}, err
	}

	past := []string{merged + "..HEAD"}

	total, err := r.countCommits(ctx, past)
	if err != nil || total == 0 {
		return AfterMerge{}, err
	}

	return r.ownAfter(ctx, past, base, total)
}

// ownAfter is AfterMerge for the total commits in past, the range past the
// merged commit, told apart by base where this clone holds it.
func (r Repository) ownAfter(ctx context.Context, past []string, base string, total int) (AfterMerge, error) {
	held := false

	if base != "" {
		var err error

		held, err = r.holdsCommit(ctx, "refs/remotes/"+base)
		if err != nil {
			return AfterMerge{}, err
		}
	}

	after := AfterMerge{Own: total}

	if held {
		past = append(past, "^refs/remotes/"+base)

		own, err := r.countCommits(ctx, past)
		if err != nil {
			return AfterMerge{}, err
		}

		after = AfterMerge{Own: own, OnBase: total - own, Base: base}
	}

	if after.Own == 0 {
		return after, nil
	}

	args := append([]string{"-C", r.dir, "log", "-z", "--max-count=" + strconv.Itoa(NamedAfterMerge), logFormat}, past...)

	out, err := r.run(ctx, gitProgram, args...)
	if err != nil {
		return AfterMerge{}, fmt.Errorf("listing the commits made after the merge: %w", err)
	}

	after.Latest = parseLog(string(out))

	return after, nil
}

// countCommits is how many commits the revisions select.
func (r Repository) countCommits(ctx context.Context, revisions []string) (int, error) {
	out, err := r.run(ctx, gitProgram, append([]string{"-C", r.dir, "rev-list", "--count"}, revisions...)...)
	if err != nil {
		return 0, fmt.Errorf("counting the commits made after the merge: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("counting the commits made after the merge: %w: %q", ErrNotACount, out)
	}

	return count, nil
}

// holdOrFetch makes sure this clone holds merged, fetching it through fetch
// when it does not, and says why not otherwise: ErrNotFetched for a commit
// the fetch failed to bring, and the look's own failure where git could not
// say whether it holds it.
func (r Repository) holdOrFetch(ctx context.Context, merged string, fetch func(context.Context) error) error {
	held, err := r.holdsCommit(ctx, merged)
	if err != nil || held {
		return err
	}

	err = fetch(ctx)
	if err != nil {
		return fmt.Errorf("the commit it merged at, %s, %w: %w", merged, ErrNotFetched, err)
	}

	held, err = r.holdsCommit(ctx, merged)
	if err != nil || held {
		return err
	}

	return fmt.Errorf("the commit it merged at, %s, %w: the fetch did not bring it", merged, ErrNotFetched)
}

// holdsCommit reports whether this clone holds the commit revision names:
// false only when git answers that it does not, and readFailure's error for a
// look that says nothing either way — one that timed out, found no git, or ran outside
// a repository — so nothing is fetched, or counted, on a guess.
func (r Repository) holdsCommit(ctx context.Context, revision string) (bool, error) {
	_, err := r.run(ctx, gitProgram, "-C", r.dir, "rev-parse", "--verify", "--quiet", revision+"^{commit}")
	if err == nil {
		return true, nil
	}

	exit, ok := errors.AsType[*proc.ExitError](err)
	if ok && exit.Code == noSuchRef {
		return false, nil
	}

	return false, readFailure(ctx, r.run, r.dir, "looking for "+revision, err)
}
