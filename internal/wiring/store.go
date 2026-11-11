// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/sanitize"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/store"
)

// ReadOnlyStore is the store's seams for a dry run: they read what an earlier
// session kept, when the store is already on disk, and never create or write
// it. A dry run posts nothing, so it claims nothing either.
func (e Environment) ReadOnlyStore(cfg config.Config, where Workspace) seams.Store {
	bound := storeDeps(e.onDisk(cfg).ReadOnly(), cfg, where)
	bound.ClaimAnnounce, bound.ConfirmAnnounce, bound.ReleaseAnnounce, bound.ReclaimAnnounce = nil, nil, nil, nil

	return bound
}

// onDisk is the store in the environment's state directory, as the
// configuration keeps it or disables it.
func (e Environment) onDisk(cfg config.Config) store.Store {
	dir, _ := e.StateDir()

	return store.New(dir, cfg.Store.Disabled)
}

// storeDeps binds the on-disk store to this repository, so the interface can open
// on what was done here before. A store with nowhere to keep its file, or one the
// configuration disabled, no-ops through the same seams, so the interface simply
// learns nothing.
func storeDeps(kept store.Store, cfg config.Config, where Workspace) seams.Store {
	repo := repoKey(where)
	instance := instanceKey(cfg.Jira.BaseURL)
	claims := announceClaims{kept: kept, repo: repo}

	return bindKept(kept, where, seams.Store{
		LastScope: func(ctx context.Context) (string, bool) {
			scope, found, _ := kept.LastScope(ctx, repo)

			// The store is a file on disk: what it reads back is untrusted, so
			// neutralize any terminal control it may carry before the composer shows it.
			return sanitize.Line(scope), found
		},
		RecordScope: func(ctx context.Context, scope string) error {
			return kept.RecordScope(ctx, repo, sanitize.Line(scope), time.Now())
		},
		Announced:       claims.announced,
		ClaimAnnounce:   claims.claim,
		ConfirmAnnounce: claims.confirm,
		ReleaseAnnounce: claims.release,
		ReclaimAnnounce: claims.reclaim,
		CachedIssues: func(ctx context.Context, view string) ([]jira.Issue, bool) {
			cached, found, _ := kept.CachedIssues(ctx, instance, view)
			if !found {
				return nil, false
			}

			return cacheableRows(fromCachedIssues(cached))
		},
		CacheIssues: func(ctx context.Context, view string, issues []jira.Issue) error {
			rows, keep := cacheableRows(issues)
			if !keep {
				return nil
			}

			return kept.CacheIssues(ctx, instance, view, toCachedIssues(rows), time.Now())
		},
	})
}

// announceClaims are the store's claims on announcements in one repository,
// keyed by repo, its credential-free key.
type announceClaims struct {
	kept store.Store
	repo string
}

// announced is every announcement claimed in the repository at a moment this
// build knows, or why the store could not be read.
func (c announceClaims) announced(ctx context.Context) ([]loop.AnnounceRecord, error) {
	recorded, err := c.kept.Announces(ctx, c.repo)
	if err != nil {
		return nil, fmt.Errorf("reading what was announced: %w", err)
	}

	return knownAnnouncements(recorded), nil
}

// claim claims made before it is posted.
func (c announceClaims) claim(ctx context.Context, made loop.Announced) (loop.Claim, error) {
	claim, err := c.kept.ClaimAnnounce(ctx, c.repo, storedAnnounce(made), time.Now())

	return claimOf(claim), err
}

// confirm records that made's post, claimed at generation, went out.
func (c announceClaims) confirm(ctx context.Context, made loop.Announced, generation int) error {
	return c.kept.ConfirmAnnounce(ctx, c.repo, storedAnnounce(made), generation, time.Now())
}

// release gives up made's first claim, at generation, whose post surely did
// not go out.
func (c announceClaims) release(ctx context.Context, made loop.Announced, generation int) error {
	return c.kept.ReleaseAnnounce(ctx, c.repo, storedAnnounce(made), generation)
}

// reclaim claims made again, from generation seen.
func (c announceClaims) reclaim(ctx context.Context, made loop.Announced, seen int) (loop.Claim, error) {
	claim, err := c.kept.ReclaimAnnounce(ctx, c.repo, storedAnnounce(made), seen, time.Now())

	return claimOf(claim), err
}

// storedAnnounce is made as the store keys it: the moment an opaque number.
func storedAnnounce(made loop.Announced) store.Announce {
	return store.Announce{Pull: made.Pull, Moment: int(made.Moment)}
}

// claimOf is the store's claim as the loop reads it.
func claimOf(claim store.Claim) loop.Claim {
	// A map, not a switch, so there is no last-case arm gobco can never see;
	// a kind the map does not hold reads as held, erring toward not posting.
	kinds := map[store.ClaimKind]loop.ClaimKind{
		store.ClaimHeld:    loop.ClaimHeld,
		store.ClaimTaken:   loop.ClaimTaken,
		store.ClaimNotKept: loop.ClaimNotKept,
	}

	return loop.Claim{
		Kind: kinds[claim.Kind], Generation: claim.Generation, ClaimedAt: claim.ClaimedAt, Posted: claim.Posted,
	}
}

// knownAnnouncements are the announcements claimed at a moment this build
// knows, posted or begun and never confirmed. The store is a file on disk, and
// a row it holds at any other moment stands for no announcement this build
// could make, so it is dropped.
func knownAnnouncements(recorded []store.AnnounceRecord) []loop.AnnounceRecord {
	made := make([]loop.AnnounceRecord, 0, len(recorded))

	for _, record := range recorded {
		moment, known := knownMoment(record.Moment)
		if known {
			made = append(made, loop.AnnounceRecord{
				Made:       loop.Announced{Pull: record.Pull, Moment: moment},
				Generation: record.Generation, ClaimedAt: record.ClaimedAt, Posted: record.Posted,
			})
		}
	}

	return made
}

// knownMoment is the moment a stored one stands for, and whether this build
// knows it.
func knownMoment(stored int) (messaging.Moment, bool) {
	moment := messaging.Moment(stored)

	switch moment {
	case messaging.MomentReady, messaging.MomentMerged:
		return moment, true
	}

	return 0, false
}

// instanceKey identifies a Jira instance for the store without keeping its URL:
// a hash of the base URL, empty when none is configured.
func instanceKey(baseURL string) string {
	if baseURL == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(baseURL))

	return hex.EncodeToString(sum[:])
}
