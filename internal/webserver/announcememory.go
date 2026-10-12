// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// errAnnouncedAlready is an announcement made already at its moment, from
// here or from a terminal.
var errAnnouncedAlready = errors.New("announced already at this moment")

// announcedCache is what the store remembers announcing in this repository,
// from any surface, read again once the forge's interval has passed — as the
// review it is told against is — so an announcement made from a terminal
// shows within one. One made here is added at once. A dry run reads nothing
// from the cache, so it holds nothing. Its lock is its own, since every open
// stream reads it.
type announcedCache struct {
	mu     sync.Mutex
	held   bool
	readAt time.Time
	made   []loop.Announced
}

// recordedAnnouncements is every announcement the store remembers, read again
// once the forge's interval has passed, or none without a store or under a dry
// run. Every stream and every post reads the one held, and the store's read
// takes a failure for none, so it runs to its end whoever asked it leaves: one
// canceled midway would be held as nothing announced, and let a post repeat
// one made. It ends when the server stops, which posts nothing more.
func (s *server) recordedAnnouncements(ctx context.Context) []loop.Announced {
	if s.deps.Store.Announced == nil || s.info.DryRun {
		return nil
	}

	s.announced.mu.Lock()
	defer s.announced.mu.Unlock()

	now := s.now()
	if !s.announced.held || now.Sub(s.announced.readAt) >= s.forgeInterval() {
		s.announced.made, s.announced.held, s.announced.readAt = s.readAnnounced(ctx), true, now
	}

	return slices.Clone(s.announced.made)
}

// readAnnounced is the store's read of what was announced, under a context
// that goes on when ctx ends and ends when the server stops.
func (s *server) readAnnounced(ctx context.Context) []loop.Announced {
	shared, release := s.worlds.outliving(ctx)
	defer release()

	return s.deps.Store.Announced(shared)
}

// recordAnnouncement remembers an announcement just made, in the store and in
// what the next frame tells, which tells it whether the store kept it or not,
// since it was made; it says why the store could not. It is recorded under
// ctx.
func (s *server) recordAnnouncement(ctx context.Context, made loop.Announced) error {
	if s.deps.Store.RecordAnnounce == nil {
		return nil
	}

	err := s.deps.Store.RecordAnnounce(ctx, made)

	s.announced.mu.Lock()
	defer s.announced.mu.Unlock()

	s.announced.made = append(s.announced.made, made)

	return err
}

// delivered is how a delivery went, as an announcement, and the warning to
// say of it: one posted that the store could not remember was made all the
// same, so its failure is noted and warned of rather than answered as one.
// Why the store could not goes to the log alone: its error can name the file.
func (s *server) delivered(err error) (string, error) {
	if errors.Is(err, loop.ErrNotRemembered) {
		s.unexpected(err)

		return loop.NotRememberedWarning, nil
	}

	return "", err
}

// announcedAlready reports that made was announced already, from any surface,
// as read under ctx.
func (s *server) announcedAlready(ctx context.Context, made loop.Announced) bool {
	return loop.AnnounceMemory{Recorded: s.recordedAnnouncements}.Holds(ctx, made)
}

// reviewAnnounced reports that the pull request the forge read was announced
// at the moment it is at now, by the rule the terminal and the command line
// read it by: never for one closed without merging, which the loop does not
// follow, so the review and the announce stage agree. What was announced is
// read under ctx.
func (s *server) reviewAnnounced(ctx context.Context, read forgeRead) bool {
	return loop.AnnounceMemory{Recorded: s.recordedAnnouncements}.HoldsNow(ctx, read.pull, read.found, read.ci)
}

// deliver posts post, and records what it made, unless that was made already,
// answering the warning to say of a post the store could not remember. The
// check, the post and the record are one step under delivering, so two asks
// at once — two tabs, or a held announcement and one made now — post it once,
// and the second learns it was made. It posts under ctx.
func (s *server) deliver(ctx context.Context, post announcePost) (string, error) {
	s.delivering.Lock()
	defer s.delivering.Unlock()

	if s.announcedAlready(ctx, post.delivery.Made) {
		return "", errAnnouncedAlready
	}

	return s.delivered(loop.Deliver(ctx, s.deps.Messaging.Post, post.memory, post.delivery))
}

// announcedBefore refuses to announce pull at a moment it was announced at
// already.
func (s *server) announcedBefore(pull int) api.Problem {
	return problem(api.ProblemCodeConflict,
		s.pullName(pull)+" was already announced at this moment, here or from a terminal")
}
