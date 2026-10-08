// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
)

// forgeCache is the forge's part of a frame — the branch's pull request, its
// reviews and its CI — held for every stream alike, so the forge is asked at
// most once an interval however many pages are open, while the repository is
// read on every frame. It holds the answer for one branch at one head commit:
// a frame for another asks at once. A read that fails keeps the answer held
// for the same branch and head until the next read is due, and a CI read that
// fails keeps the CI held for the same pull request; either way the failure is
// held beside the answer, so every frame until the next read says so. The
// forge is read outside its lock, one read of a branch at a time: a frame that
// finds one under way is served the answer held for its branch at once, or,
// with none held, waits for that read rather than show none found, so one
// slow forge holds up no other stream that has an answer to show.
type forgeCache struct {
	mu     sync.Mutex
	held   bool
	key    forgeKey
	readAt time.Time
	read   forgeRead
	// failed is why the last read did not answer, or nil when it did.
	failed error
	// reading is each read under way, by what it reads for, closed once it
	// lands.
	reading map[forgeKey]chan struct{}
	// dropped counts the drops, so a read that began before one lands due.
	dropped int
}

// forgeKey is what a forge answer was read for: a branch, by name, at a head.
type forgeKey struct {
	branch string
	head   string
}

// forgeTurn is what the cache gives a frame asking of a branch: the answer
// held and its failure, a read under way to wait for, or, with neither, the
// go-ahead to read, with the drops counted as it began.
type forgeTurn struct {
	answered bool
	read     forgeRead
	failed   error
	landing  <-chan struct{}
	dropped  int
}

// forgeReview is what the forge cache holds of the branch, read again when the
// cache holds none for the branch at its head, or once the forge interval has
// passed since the last read, with why the last read failed, if it did.
func (s *server) forgeReview(branch gitrepo.Branch) (forgeRead, error) {
	key := forgeKey{branch: branch.Name, head: branch.Head}

	for {
		turn := s.forgeAnswer.serve(key, s.now(), s.forgeInterval())

		switch {
		case turn.answered:
			return turn.read, turn.failed
		case turn.landing != nil:
			<-turn.landing
		default:
			read, err := s.readForge(branch)

			return s.forgeAnswer.land(key, turn.dropped, s.now(), read, err)
		}
	}
}

// serve answers what the cache holds for key while it is fresh, and while a
// read of key is under way; it hands back that read to wait for when nothing
// is held for key, and otherwise starts the caller's own read, which land
// ends.
func (c *forgeCache) serve(key forgeKey, now time.Time, interval time.Duration) forgeTurn {
	c.mu.Lock()
	defer c.mu.Unlock()

	held := c.held && c.key == key
	landing, reading := c.reading[key]

	if held && (reading || now.Sub(c.readAt) < interval) {
		return forgeTurn{answered: true, read: c.read, failed: c.failed}
	}

	if reading {
		return forgeTurn{landing: landing}
	}

	if c.reading == nil {
		c.reading = map[forgeKey]chan struct{}{}
	}

	c.reading[key] = make(chan struct{})

	return forgeTurn{dropped: c.dropped}
}

// land holds what the read of key began after dropped drops found, and lets
// go every frame waiting on it. A read a drop came during is held already
// due, since it may say less than the forge does now. A read that failed
// keeps the answer held for key, and a CI read that failed keeps the CI held
// for the same pull request. It answers what the frame shows.
func (c *forgeCache) land(key forgeKey, dropped int, now time.Time, read forgeRead, err error) (forgeRead, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.reading[key])
	delete(c.reading, key)

	held := c.held && c.key == key
	c.readAt, c.failed = now, err

	if c.dropped != dropped {
		c.readAt = time.Time{}
	}

	if err != nil && held {
		return c.read, err
	}

	if held && read.ciErr != nil && c.holdsPull(read.pull.Number) {
		read.ci, read.ciRead = c.read.ci, c.read.ciRead
	}

	c.held, c.key, c.read = true, key, read

	return read, err
}

// drop forgets the held answer, so the next frame asks the forge: a write here
// that changes what the forge would say shows on the next frame rather than an
// interval later, even when a read was under way as it was made.
func (c *forgeCache) drop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.held = false
	c.dropped++
}

// holdsPull reports whether the held answer is about the pull request of this
// number, so the CI held for it can stand in for a CI read that failed; the CI
// of another pull request, or of none, cannot. The caller holds the lock.
func (c *forgeCache) holdsPull(number int) bool {
	return c.read.found && c.read.pull.Number == number
}

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

// issuesTurn is what the cache gives a frame asking for a view: the page
// held, a search under way to wait for, or, with neither, the go-ahead to
// search, with the drops counted as it began.
type issuesTurn struct {
	answered bool
	page     heldPage
	landing  <-chan struct{}
	dropped  int
}

// frameIssues is the first page of the issues jql finds, from the issues
// cache, searched again once it is older than the forge's interval.
func (s *server) frameIssues(jql string) (jira.SearchResult, error) {
	for {
		turn := s.issuesHeld.serve(jql, s.now(), s.forgeInterval())

		switch {
		case turn.answered:
			return turn.page.result, turn.page.failed
		case turn.landing != nil:
			<-turn.landing
		default:
			result, err := s.deps.Search(jql, 0)
			s.issuesHeld.land(jql, turn.dropped, heldPage{result: result, failed: err, readAt: s.now()})

			return result, err
		}
	}
}

// serve answers the page held for jql while it is fresh, and while a search
// of it is under way; it hands back that search to wait for when nothing is
// held, and otherwise starts the caller's own search, which land ends.
func (c *issuesCache) serve(jql string, now time.Time, interval time.Duration) issuesTurn {
	c.mu.Lock()
	defer c.mu.Unlock()

	page, held := c.pages[jql]
	landing, reading := c.reading[jql]

	if held && (reading || now.Sub(page.readAt) < interval) {
		return issuesTurn{answered: true, page: page}
	}

	if reading {
		return issuesTurn{landing: landing}
	}

	if c.reading == nil {
		c.reading, c.pages = map[string]chan struct{}{}, map[string]heldPage{}
	}

	c.reading[jql] = make(chan struct{})

	return issuesTurn{dropped: c.dropped}
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

// assignedInterval is how long the tracker's answer to which of the branches'
// issues are yours serves every stream: an issue is reassigned or finished
// rarely, and each answer is a search of the tracker.
const assignedInterval = time.Minute

// assignedCache is which of the branches' issues the tracker last said are
// yours, held for every stream alike and asked again when the branches name
// other issues or assignedInterval has passed. An ask that fails keeps the last
// answer until the next is due, and an issue that answer never covered counts
// as yours, as every issue does with no answer yet, or no tracker to ask, so
// the list never waits on the tracker. Its lock is held across the ask, so
// streams that find one due together make one.
type assignedCache struct {
	mu     sync.Mutex
	held   bool
	keys   []jira.Key
	readAt time.Time
	// mine is the tracker's last answer, and answered the keys it was asked
	// about, sorted; mine is nil until the tracker has answered.
	mine     map[jira.Key]bool
	answered []jira.Key
}

// yours is which of keys are yours by the last answer: those it said are, and
// those it never covered; nil, counting every issue, before any answer. The
// caller holds the lock.
func (c *assignedCache) yours(keys []jira.Key) map[jira.Key]bool {
	if c.mine == nil {
		return nil
	}

	yours := maps.Clone(c.mine)

	for _, key := range keys {
		if _, covered := slices.BinarySearch(c.answered, key); !covered {
			yours[key] = true
		}
	}

	return yours
}

// yourIssues is which of keys name your issues, from the assigned cache: nil,
// counting every issue, with no tracker to ask or none that has answered.
func (s *server) yourIssues(keys []jira.Key) map[jira.Key]bool {
	if s.deps.SearchLenient == nil {
		return nil
	}

	asked := slices.Compact(slices.Sorted(slices.Values(keys)))

	s.assigned.mu.Lock()
	defer s.assigned.mu.Unlock()

	now := s.now()
	if s.assigned.held && slices.Equal(s.assigned.keys, asked) && now.Sub(s.assigned.readAt) < assignedInterval {
		return s.assigned.yours(asked)
	}

	mine, err := loop.AssignedKeys(s.deps.SearchLenient, asked)
	s.assigned.held, s.assigned.keys, s.assigned.readAt = true, asked, now

	if err == nil {
		s.assigned.mine, s.assigned.answered = mine, asked
	}

	return s.assigned.yours(asked)
}
