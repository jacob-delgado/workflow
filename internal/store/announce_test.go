// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/store"
)

// storedTime is a claim time as a row on disk holds it.
const storedTime = "2026-09-22T12:00:00Z"

// ready is the announcement the claim tests post: #42, ready for review.
func ready() store.Announce {
	return store.Announce{Pull: 42, Moment: 1}
}

// claimed is a store with ready claimed in it, and the claim it took.
func claimed(t *testing.T) (store.Store, store.Claim) {
	t.Helper()

	kept := store.New(t.TempDir(), false)

	claim, err := kept.ClaimAnnounce(t.Context(), repo, ready(), theTime())
	if err != nil || claim.Kind != store.ClaimTaken {
		t.Fatalf("ClaimAnnounce = %+v, %v; want it taken", claim, err)
	}

	return kept, claim
}

// records reads every announcement kept for repo, failing the test on an
// error.
func records(t *testing.T, kept store.Store) []store.AnnounceRecord {
	t.Helper()

	got, err := kept.Announces(t.Context(), repo)
	if err != nil {
		t.Fatalf("Announces returned %v, want nil", err)
	}

	return got
}

func TestAClaimIsKeptUnconfirmedUntilConfirmed(t *testing.T) {
	t.Parallel()

	// Arrange
	kept, claim := claimed(t)

	// Act
	got := records(t, kept)

	// Assert
	want := store.AnnounceRecord{Announce: ready(), Generation: 1, ClaimedAt: theTime(), Posted: false}
	if claim.Generation != 1 || len(got) != 1 || got[0] != want {
		t.Errorf("claim %+v, Announces = %+v; want generation 1, kept unconfirmed as %+v", claim, got, want)
	}
}

func TestAConfirmedClaimReadsAsPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	kept, claim := claimed(t)

	// Act
	err := kept.ConfirmAnnounce(t.Context(), repo, ready(), claim.Generation, theTime().Add(time.Second))

	// Assert
	got := records(t, kept)
	if err != nil || len(got) != 1 || !got[0].Posted {
		t.Errorf("ConfirmAnnounce = %v, Announces = %+v; want it posted", err, got)
	}
}

func TestASecondClaimIsHeld(t *testing.T) {
	t.Parallel()

	// Arrange
	kept, _ := claimed(t)

	// Act
	again, err := kept.ClaimAnnounce(t.Context(), repo, ready(), theTime().Add(time.Minute))

	// Assert
	want := store.Claim{Kind: store.ClaimHeld, Generation: 1, ClaimedAt: theTime(), Posted: false}
	if err != nil || again != want {
		t.Errorf("a second ClaimAnnounce = %+v, %v; want %+v", again, err, want)
	}
}

func TestAClaimIsKeptPerMoment(t *testing.T) {
	t.Parallel()

	// Arrange
	// The same pull request announced at two moments — opened, then merged — is
	// two claims, so neither keeps the other from going.
	kept, _ := claimed(t)

	// Act
	merged, err := kept.ClaimAnnounce(t.Context(), repo, store.Announce{Pull: 42, Moment: 3}, theTime())

	// Assert
	if err != nil || merged.Kind != store.ClaimTaken || len(records(t, kept)) != 2 {
		t.Errorf("claiming the merge = %+v, %v; want it taken beside the first", merged, err)
	}
}

// claimTally is how many claims came back of each kind, and their errors.
type claimTally struct {
	mu    sync.Mutex
	kinds map[store.ClaimKind]int
	errs  []error
}

// claim claims ready in kept, counting what came back.
func (c *claimTally) claim(kept store.Store) {
	got, err := kept.ClaimAnnounce(context.Background(), repo, ready(), theTime())

	c.mu.Lock()
	defer c.mu.Unlock()

	c.kinds[got.Kind]++

	if err != nil {
		c.errs = append(c.errs, err)
	}
}

func TestOnlyOneOfManyClaimsAcrossTwoStoresIsTaken(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two stores on one directory, as two processes would be, each claiming
	// the same announcement from eight goroutines at once.
	dir := t.TempDir()
	first, second := store.New(dir, false), store.New(dir, false)

	err := first.RecordScope(t.Context(), repo, recorded, theTime())
	if err != nil {
		t.Fatalf("making the store: %v", err)
	}

	var wait sync.WaitGroup

	tally := &claimTally{kinds: map[store.ClaimKind]int{}}

	// Act
	for range 8 {
		wait.Go(func() { tally.claim(first) })
		wait.Go(func() { tally.claim(second) })
	}

	wait.Wait()

	// Assert
	if len(tally.errs) != 0 || tally.kinds[store.ClaimTaken] != 1 || tally.kinds[store.ClaimHeld] != 15 {
		t.Errorf("16 claims gave %d taken and %d held, errors %v; want exactly one taken",
			tally.kinds[store.ClaimTaken], tally.kinds[store.ClaimHeld], tally.errs)
	}
}

func TestAReleaseRemovesOnlyItsOwnFirstUnconfirmedClaim(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		confirm    bool
		generation int
		wantKept   bool
	}{
		"its own unconfirmed claim":  {confirm: false, generation: 1, wantKept: false},
		"another generation's claim": {confirm: false, generation: 2, wantKept: true},
		"a confirmed post":           {confirm: true, generation: 1, wantKept: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept, claim := claimed(t)
			if tt.confirm {
				_ = kept.ConfirmAnnounce(t.Context(), repo, ready(), claim.Generation, theTime())
			}

			// Act
			err := kept.ReleaseAnnounce(t.Context(), repo, ready(), tt.generation)

			// Assert
			if got := len(records(t, kept)) == 1; err != nil || got != tt.wantKept {
				t.Errorf("ReleaseAnnounce = %v, kept %v; want kept %v", err, got, tt.wantKept)
			}
		})
	}
}

func TestAReclaimIsACompareAndSwap(t *testing.T) {
	t.Parallel()

	// Arrange
	// Two sessions both saw generation 1 and both ask to post again.
	kept, claim := claimed(t)
	later := theTime().Add(time.Hour)

	first, err := kept.ReclaimAnnounce(t.Context(), repo, ready(), claim.Generation, later)
	if err != nil {
		t.Fatalf("the first ReclaimAnnounce returned %v", err)
	}

	// Act
	second, err := kept.ReclaimAnnounce(t.Context(), repo, ready(), claim.Generation, later)

	// Assert
	if err != nil || first.Kind != store.ClaimTaken || first.Generation != 2 || second.Kind != store.ClaimHeld {
		t.Errorf("reclaims = %+v then %+v, %v; want the first taken at generation 2 and the second held",
			first, second, err)
	}
}

func TestARefusedRepostLeavesAConfirmedPostPosted(t *testing.T) {
	t.Parallel()

	// Arrange
	// A post confirmed, then posted again by asking, which the service
	// refuses: the claim it took is released.
	kept, claim := claimed(t)
	_ = kept.ConfirmAnnounce(t.Context(), repo, ready(), claim.Generation, theTime())

	again, err := kept.ReclaimAnnounce(t.Context(), repo, ready(), claim.Generation, theTime().Add(time.Hour))
	if err != nil || again.Kind != store.ClaimTaken {
		t.Fatalf("ReclaimAnnounce = %+v, %v; want it taken", again, err)
	}

	// Act
	err = kept.ReleaseAnnounce(t.Context(), repo, ready(), again.Generation)

	// Assert
	got := records(t, kept)
	if err != nil || len(got) != 1 || !got[0].Posted {
		t.Errorf("ReleaseAnnounce = %v, Announces = %+v; want the earlier post still posted", err, got)
	}
}

func TestAReclaimOfAClaimGoneClaimsAfresh(t *testing.T) {
	t.Parallel()

	// Arrange
	// What was claimed is gone, as a db-clean leaves it.
	kept := store.New(t.TempDir(), false)

	// Act
	got, err := kept.ReclaimAnnounce(t.Context(), repo, ready(), 1, theTime())

	// Assert
	if err != nil || got.Kind != store.ClaimTaken || got.Generation != 1 {
		t.Errorf("ReclaimAnnounce = %+v, %v; want a fresh claim taken at generation 1", got, err)
	}
}

func TestAStoreThatKeepsNothingTakesNoClaim(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cases := map[string]struct {
		kept store.Store
		repo string
	}{
		"a disabled store":            {kept: store.New(dir, true), repo: repo},
		"a read-only store":           {kept: store.New(dir, false).ReadOnly(), repo: repo},
		"a repository without a name": {kept: store.New(dir, false), repo: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			claim, claimErr := tt.kept.ClaimAnnounce(t.Context(), tt.repo, ready(), theTime())
			reclaim, reclaimErr := tt.kept.ReclaimAnnounce(t.Context(), tt.repo, ready(), 1, theTime())

			// Assert
			if claimErr != nil || claim.Kind != store.ClaimNotKept || reclaimErr != nil ||
				reclaim.Kind != store.ClaimNotKept {
				t.Errorf("ClaimAnnounce = %+v, %v; ReclaimAnnounce = %+v, %v; want neither kept",
					claim, claimErr, reclaim, reclaimErr)
			}
		})
	}
}

func TestADisabledStoreConfirmsAndReleasesNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	off := store.New(t.TempDir(), true)

	// Act
	confirmErr := off.ConfirmAnnounce(t.Context(), repo, ready(), 1, theTime())
	releaseErr := off.ReleaseAnnounce(t.Context(), repo, ready(), 1)

	// Assert
	if confirmErr != nil || releaseErr != nil || len(records(t, off)) != 0 {
		t.Errorf("a disabled store answered %v, %v; want it to no-op", confirmErr, releaseErr)
	}
}

func TestAnAnnouncementKeptByTheFirstSchemaIsForgotten(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file the first schema made, announcing #42 with no claim.
	dir := t.TempDir()
	seedDatabaseAt(t, dir, 1,
		table(`CREATE TABLE announces (
			repo TEXT NOT NULL, pull INTEGER NOT NULL, moment INTEGER NOT NULL, announced_at TEXT NOT NULL,
			PRIMARY KEY (repo, pull, moment)
		) STRICT`),
		statement{
			query: `INSERT INTO announces (repo, pull, moment, announced_at) VALUES (?, ?, ?, ?)`,
			args:  []any{repo, 42, 1, storedTime},
		},
	)

	// Act
	claim, err := store.New(dir, false).ClaimAnnounce(t.Context(), repo, ready(), theTime())

	// Assert
	if err != nil || claim.Kind != store.ClaimTaken {
		t.Errorf("ClaimAnnounce over the first schema's file = %+v, %v; want it taken afresh", claim, err)
	}

	if got := readPragma(t, dir, "user_version"); got != currentVersion {
		t.Errorf("the file is at version %d, want %d", got, currentVersion)
	}
}

func TestAMalformedClaimIsLeftOut(t *testing.T) {
	t.Parallel()

	// Arrange
	// Rows no build of the store writes: a generation below one, and a time
	// that is not one.
	dir := t.TempDir()
	seedDatabase(t, dir,
		table(`CREATE TABLE announces (
			repo TEXT NOT NULL, pull INTEGER NOT NULL, moment INTEGER NOT NULL, claim INTEGER NOT NULL,
			claimed_at TEXT NOT NULL, posted_at TEXT, PRIMARY KEY (repo, pull, moment)
		) STRICT`),
		statement{
			query: `INSERT INTO announces (repo, pull, moment, claim, claimed_at) VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?)`,
			args:  []any{repo, 42, 1, 0, storedTime, repo, 43, 1, 1, "yesterday"},
		},
	)

	// Act
	got, err := store.New(dir, false).Announces(t.Context(), repo)

	// Assert
	if err != nil || len(got) != 0 {
		t.Errorf("Announces = %+v, %v; want both malformed rows left out", got, err)
	}
}
