// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// issuesCache is the first page of each view's issues as the tracker last
// answered it, held for every stream alike and searched again once the
// forge's interval has passed, so however many pages are open the tracker is
// searched once an interval for a view, not once a frame for each. A search
// that fails is held with its error, so a failing tracker is not asked every
// frame either. One search of a view runs at a time, outside the lock: a
// frame that finds one under way is served what is held for the view at
// once, or, with nothing held, waits for that search. A write here that
// changes what the tracker answers drops every view, so the next frame
// searches again. Its lock is its own, since every open stream reads it.
type issuesCache struct {
	mu    sync.Mutex
	pages map[string]heldPage
	// reading is each search under way, by its JQL, closed once it lands.
	reading map[string]chan struct{}
	// dropped counts the drops, so a search that began before one lands due.
	dropped int
}

// heldPage is a view's first page as a search answered it, or why the search
// failed, and when it was made.
type heldPage struct {
	result jira.SearchResult
	failed error
	readAt time.Time
}

// frameIssues is the first page of the issues jql finds, from the issues
// cache, searched again once it is older than the forge's interval. Every
// stream shares the search, so it runs to its end even if the page whose
// frame, under ctx, began it leaves: one canceled then would be held as failed
// for every page until the interval passed. It ends when the server stops.
func (s *server) frameIssues(ctx context.Context, jql string) (jira.SearchResult, error) {
	return sharedRead(ctx,
		func() turn[jira.SearchResult] { return s.issuesHeld.serve(jql, s.now(), s.forgeInterval()) },
		func(dropped int) (jira.SearchResult, error) {
			shared, release := s.worlds.outliving(ctx)
			defer release()

			result, err := s.deps.Jira.Search(shared, jql, 0)
			s.issuesHeld.land(jql, dropped, heldPage{result: result, failed: err, readAt: s.now()})

			return result, err
		},
	)
}

// serve answers the page held for jql while it is fresh, and while a search
// of it is under way; it hands back that search to wait for when nothing is
// held, and otherwise starts the caller's own search, which land ends.
func (c *issuesCache) serve(jql string, now time.Time, interval time.Duration) turn[jira.SearchResult] {
	c.mu.Lock()
	defer c.mu.Unlock()

	page, held := c.pages[jql]
	landing, reading := c.reading[jql]

	if held && (reading || now.Sub(page.readAt) < interval) {
		return turn[jira.SearchResult]{answered: true, value: page.result, failed: page.failed}
	}

	if reading {
		return turn[jira.SearchResult]{landing: landing}
	}

	if c.reading == nil {
		c.reading, c.pages = map[string]chan struct{}{}, map[string]heldPage{}
	}

	c.reading[jql] = make(chan struct{})

	return turn[jira.SearchResult]{dropped: c.dropped}
}

// land holds the page the search of jql began after dropped drops found,
// already due when a drop came during it, and lets go every frame waiting on
// it.
func (c *issuesCache) land(jql string, dropped int, page heldPage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.reading[jql])
	delete(c.reading, jql)

	if c.dropped != dropped {
		page.readAt = time.Time{}
	}

	c.pages[jql] = page
}

// drop forgets every view's page, so the next frame searches again: a change
// made here to an issue shows on the next frame rather than an interval
// later, even when a search was under way as it was made.
func (c *issuesCache) drop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	clear(c.pages)
	c.dropped++
}
