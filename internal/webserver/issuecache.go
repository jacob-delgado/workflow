// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/report"
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
	// reading is each search under way, by its JQL.
	reading map[string]*landing[jira.SearchResult]
	// dropped counts the drops, so a search that began before one lands due.
	dropped int
	// unkept is each view's newest page a search answered that the store has
	// not been asked to keep yet, and keeping each view whose page a frame is
	// putting in the store now. That frame keeps a page landing meanwhile
	// after its own, so the store never ends on an older page than a newer
	// one it was asked to keep.
	unkept  map[string][]jira.Issue
	keeping map[string]bool
}

// heldPage is a view's first page as a search answered it, or why the search
// failed, when it was made, and whether a drop came during it: a page that
// says less than the tracker does now, which no frame is served, even while
// the next search is under way.
type heldPage struct {
	result jira.SearchResult
	failed error
	readAt time.Time
	due    bool
}

// frameIssues is the first page of the issues jql finds, from the issues
// cache, searched again once it is older than the forge's interval. Every
// stream shares the search, so it runs to its end even if the page whose
// frame, under ctx, began it leaves: one canceled then would be held as failed
// for every page until the interval passed. It ends when the server stops. A
// search that answered keeps its page in the store for the next session, once
// it is held, so no other stream waits on the store's write.
func (s *server) frameIssues(ctx context.Context, jql string) (jira.SearchResult, error) {
	return sharedRead(ctx,
		func() turn[jira.SearchResult] { return s.issuesHeld.serve(jql, s.now(), s.forgeInterval()) },
		func(dropped int) (jira.SearchResult, error) {
			shared, release := s.worlds.outliving(ctx)
			defer release()

			result, err := s.deps.Jira.Search(shared, jql, 0)
			s.issuesHeld.land(jql, dropped, heldPage{result: result, failed: err, readAt: s.now()})

			if err == nil {
				s.keepFirstPage(shared, jql)
			}

			return result, err
		},
	)
}

// keepFirstPage keeps a view's first page of issues in the store, by the
// terminal's rule: first pages only, and nothing for a search that failed, so
// either surface opens on the list the other last saw. A dry run keeps
// nothing. A store that could not keep the list is noted, since the list is
// shown all the same. It runs under the shared search's context, so it
// outlives the page that searched and ends with the server. While another
// frame keeps the view's page it returns at once, leaving its newer page to
// that frame, and the frame keeping one keeps every page landing meanwhile in
// turn, so the last the store is asked to keep is the newest.
func (s *server) keepFirstPage(ctx context.Context, jql string) {
	if s.deps.Store.CacheIssues == nil || s.info.DryRun {
		return
	}

	issues, ours := s.issuesHeld.startKeeping(jql)
	for ours {
		err := s.deps.Store.CacheIssues(ctx, jql, issues)
		if err != nil {
			s.unexpected(fmt.Errorf("keeping the issue list for the next session: %w", err))
		}

		issues, ours = s.issuesHeld.nextToKeep(jql)
	}
}

// viewSeed is the view's issues as the store last kept them, and whether
// there are any to send: there are none without a tracker to ask again, a
// store to read, outside a dry run, or a list kept for jql. The read is the
// page's own, made under its ctx. The rows go through the same mapper as a
// live page, which carries a category the contract does not name as one it
// does: a row read back from disk is not trusted to fit.
func (s *server) viewSeed(ctx context.Context, jql string) (api.Seed, bool) {
	if s.deps.Jira.Search == nil || s.deps.Store.CachedIssues == nil || s.info.DryRun {
		return api.Seed{}, false
	}

	kept, held := s.deps.Store.CachedIssues(ctx, jql)
	if !held {
		return api.Seed{}, false
	}

	return api.Seed{Issues: report.IssuesPage(jira.SearchResult{Issues: kept, Total: len(kept)}, 0)}, true
}

// serve answers the page held for jql while it is fresh, and while a search
// of it is under way, unless a drop came during the search it holds; it hands
// back that search to wait for when no page it may serve is held, and
// otherwise starts the caller's own search, which land ends.
func (c *issuesCache) serve(jql string, now time.Time, interval time.Duration) turn[jira.SearchResult] {
	c.mu.Lock()
	defer c.mu.Unlock()

	page, held := c.pages[jql]
	underWay, reading := c.reading[jql]

	if held && !page.due && (reading || now.Sub(page.readAt) < interval) {
		return turn[jira.SearchResult]{answered: true, value: page.result, failed: page.failed}
	}

	if reading {
		return turn[jira.SearchResult]{landing: underWay}
	}

	if c.reading == nil {
		c.reading, c.pages = map[string]*landing[jira.SearchResult]{}, map[string]heldPage{}
		c.unkept, c.keeping = map[string][]jira.Issue{}, map[string]bool{}
	}

	c.reading[jql] = newLanding[jira.SearchResult]()

	return turn[jira.SearchResult]{dropped: c.dropped}
}

// land holds the page the search of jql began after dropped drops found,
// already due when a drop came during it, and lets go every frame waiting on
// it.
func (c *issuesCache) land(jql string, dropped int, page heldPage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.reading[jql].end()
	delete(c.reading, jql)

	page.due = c.dropped != dropped
	c.pages[jql] = page

	if page.failed == nil {
		c.unkept[jql] = page.result.Issues
	}
}

// startKeeping hands the caller the newest page of jql the store has not
// been asked to keep, for it to keep, unless another frame is keeping the
// view's page, which then keeps this one after its own: ours is false then,
// and when there is none to keep.
func (c *issuesCache) startKeeping(jql string) ([]jira.Issue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.keeping[jql] {
		return nil, false
	}

	return c.takeUnkept(jql)
}

// nextToKeep is the page of jql that landed while the caller kept the one
// before, for it to keep next, or false once none did, when the view is left
// for the next frame to keep.
func (c *issuesCache) nextToKeep(jql string) ([]jira.Issue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.takeUnkept(jql)
}

// takeUnkept takes the newest page of jql the store has not been asked to
// keep, and marks the view as being kept while there is one. It runs under
// mu.
func (c *issuesCache) takeUnkept(jql string) ([]jira.Issue, bool) {
	issues, waiting := c.unkept[jql]
	delete(c.unkept, jql)
	c.keeping[jql] = waiting

	return issues, waiting
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
