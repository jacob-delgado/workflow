// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
)

// Commands a branch read onto release runs.
const (
	verifyOriginRelease = "git -C /work rev-parse --verify --quiet refs/remotes/origin/release"
	verifyLocalRelease  = "git -C /work rev-parse --verify --quiet refs/heads/release"
	releaseBase         = "release"
)

// ontoRelease is featureBranch read onto release: the default base is never
// asked for, and the range is the one past origin/release.
func ontoRelease() map[string]reply {
	replies := featureBranch()
	delete(replies, originsHead)
	delete(replies, logFromMain)
	delete(replies, baseAge+originMain)

	replies[verifyOriginRelease] = reply{out: []byte("0123abcd\n")}
	replies[logCommits+"origin/release..HEAD"] = reply{out: []byte(secondHash + "\x00" + secondSubject + "\x00")}
	replies[baseAge+"origin/release"] = reply{out: []byte(baseUpdatedISO + "\n")}

	return replies
}

func TestReadBranchOntoReadsTheCommitsPastTheBaseChosen(t *testing.T) {
	t.Parallel()

	// Act
	branch, err := gitrepo.At(fakeRunner(t, ontoRelease()), workDir).ReadBranchOnto(t.Context(), releaseBase)

	// Assert
	if err != nil || branch.Base != "origin/release" || branch.BaseName() != releaseBase ||
		len(branch.Commits) != 1 || branch.Commits[0].Subject != secondSubject {
		t.Errorf("ReadBranchOnto = %+v, %v; want the one commit past origin/release", branch, err)
	}
}

func TestReadBranchOntoTakesALocalBaseNoRemoteHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := ontoRelease()
	delete(replies, logCommits+"origin/release..HEAD")
	delete(replies, baseAge+"origin/release")
	replies[verifyOriginRelease] = reply{err: errNoRef}
	replies[verifyLocalRelease] = reply{out: []byte("0123abcd\n")}
	replies[logCommits+"release..HEAD"] = reply{out: []byte(secondHash + "\x00" + secondSubject + "\x00")}
	replies[baseAge+releaseBase] = reply{out: []byte(baseUpdatedISO + "\n")}

	// Act
	branch, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranchOnto(t.Context(), releaseBase)

	// Assert
	if err != nil || branch.Base != releaseBase || len(branch.Commits) != 1 {
		t.Errorf("ReadBranchOnto = %+v, %v; want the commit past the local release", branch, err)
	}
}

func TestReadBranchOntoRefusesABaseNothingHolds(t *testing.T) {
	t.Parallel()

	// Arrange
	replies := ontoRelease()
	replies[verifyOriginRelease] = reply{err: errNoRef}
	replies[verifyLocalRelease] = reply{err: errNoRef}

	// Act
	_, err := gitrepo.At(fakeRunner(t, replies), workDir).ReadBranchOnto(t.Context(), releaseBase)

	// Assert
	if !errors.Is(err, gitrepo.ErrNoSuchBase) {
		t.Errorf("ReadBranchOnto = %v, want %v", err, gitrepo.ErrNoSuchBase)
	}
}

func TestReadBranchOntoRefusesABaseThatReadsAsAnOption(t *testing.T) {
	t.Parallel()

	// Act
	_, err := gitrepo.At(fakeRunner(t, ontoRelease()), workDir).ReadBranchOnto(t.Context(), "--orphan=x")

	// Assert
	if !errors.Is(err, gitrepo.ErrOptionLikeRef) {
		t.Errorf("ReadBranchOnto = %v, want %v", err, gitrepo.ErrOptionLikeRef)
	}
}
