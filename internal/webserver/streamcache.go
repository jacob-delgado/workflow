// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"fmt"
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

// turn is what a shared read's cache gives a frame asking of it: the answer
// held and its failure, a read under way to wait for, or, with neither, the
// go-ahead to read, with the drops counted as it began.
type turn[V any] struct {
	answered bool
	value    V
	failed   error
	landing  <-chan struct{}
	dropped  int
}

// sharedRead is a read every stream shares, made outside its cache's lock:
// what serve answers, or, when serve hands back a read under way, what serve
// answers once that read lands, or, when serve gives the go-ahead, what read
// makes of it, given the drops counted as it began.
func sharedRead[V any](serve func() turn[V], read func(dropped int) (V, error)) (V, error) {
	for {
		next := serve()

		switch {
		case next.answered:
			return next.value, next.failed
		case next.landing != nil:
			<-next.landing
		default:
			return read(next.dropped)
		}
	}
}

// forgeReview is what the forge cache holds of the branch, read again when the
// cache holds none for the branch at its head, or once the forge interval has
// passed since the last read, with why the last read failed, if it did.
func (s *server) forgeReview(branch gitrepo.Branch) (forgeRead, error) {
	key := forgeKey{branch: branch.Name, head: branch.Head}

	return sharedRead(
		func() turn[forgeRead] { return s.forgeAnswer.serve(key, s.now(), s.forgeInterval()) },
		func(dropped int) (forgeRead, error) {
			read, err := s.readForge(branch)

			return s.forgeAnswer.land(key, dropped, s.now(), read, err)
		},
	)
}

// serve answers what the cache holds for key while it is fresh, and while a
// read of key is under way; it hands back that read to wait for when nothing
// is held for key, and otherwise starts the caller's own read, which land
// ends.
func (c *forgeCache) serve(key forgeKey, now time.Time, interval time.Duration) turn[forgeRead] {
	c.mu.Lock()
	defer c.mu.Unlock()

	held := c.held && c.key == key
	landing, reading := c.reading[key]

	if held && (reading || now.Sub(c.readAt) < interval) {
		return turn[forgeRead]{answered: true, value: c.read, failed: c.failed}
	}

	if reading {
		return turn[forgeRead]{landing: landing}
	}

	if c.reading == nil {
		c.reading = map[forgeKey]chan struct{}{}
	}

	c.reading[key] = make(chan struct{})

	return turn[forgeRead]{dropped: c.dropped}
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

// frameIssues is the first page of the issues jql finds, from the issues
// cache, searched again once it is older than the forge's interval.
func (s *server) frameIssues(jql string) (jira.SearchResult, error) {
	return sharedRead(
		func() turn[jira.SearchResult] { return s.issuesHeld.serve(jql, s.now(), s.forgeInterval()) },
		func(dropped int) (jira.SearchResult, error) {
			result, err := s.deps.Search(jql, 0)
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

// assignedInterval is how long the tracker's answer to which of the branches'
// issues are yours serves every stream: an issue is reassigned or finished
// rarely, and each answer is a search of the tracker.
const assignedInterval = time.Minute

// assignedCache is which of the branches' issues the tracker last said are
// yours, held for every stream alike and asked again when the branches name
// other issues or assignedInterval has passed. An ask that fails keeps the last
// answer until the next is due, and an issue that answer never covered counts
// as yours, as every issue does with no answer yet, or no tracker to ask, so
// the list never waits on the tracker. One ask runs at a time, outside the
// lock: a frame that finds one under way is served the answer held at once,
// or, before any ask has landed, waits for that ask.
type assignedCache struct {
	mu     sync.Mutex
	held   bool
	keys   []jira.Key
	readAt time.Time
	// mine is the tracker's last answer, and answered the keys it was asked
	// about, sorted; mine is nil until the tracker has answered.
	mine     map[jira.Key]bool
	answered []jira.Key
	// reading is the ask under way, closed once it lands, or nil with none.
	reading chan struct{}
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
// counting every issue, with no tracker to ask or none that has answered. A
// failed ask is no failure here: the answer held stands in for it.
func (s *server) yourIssues(keys []jira.Key) map[jira.Key]bool {
	if s.deps.SearchLenient == nil {
		return nil
	}

	asked := slices.Compact(slices.Sorted(slices.Values(keys)))

	yours, _ := sharedRead(
		func() turn[map[jira.Key]bool] { return s.assigned.serve(asked, s.now()) },
		func(int) (map[jira.Key]bool, error) {
			mine, err := loop.AssignedKeys(s.deps.SearchLenient, asked)

			return s.assigned.land(asked, s.now(), mine, err), nil
		},
	)

	return yours
}

// serve answers which of asked are yours while the answer held was asked of
// the same keys within assignedInterval, and while an ask is under way; it
// hands back that ask to wait for before any has landed, and otherwise
// starts the caller's own ask, which land ends.
func (c *assignedCache) serve(asked []jira.Key, now time.Time) turn[map[jira.Key]bool] {
	c.mu.Lock()
	defer c.mu.Unlock()

	fresh := slices.Equal(c.keys, asked) && now.Sub(c.readAt) < assignedInterval
	if c.held && (c.reading != nil || fresh) {
		return turn[map[jira.Key]bool]{answered: true, value: c.yours(asked)}
	}

	if c.reading != nil {
		return turn[map[jira.Key]bool]{landing: c.reading}
	}

	c.reading = make(chan struct{})

	return turn[map[jira.Key]bool]{}
}

// land holds what the ask of asked made at now answered, keeping the last
// answer when it failed, and lets go every frame waiting on it. It answers
// which of asked are yours.
func (c *assignedCache) land(asked []jira.Key, now time.Time, mine map[jira.Key]bool, err error) map[jira.Key]bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.reading)
	c.reading = nil
	c.held, c.keys, c.readAt = true, asked, now

	if err == nil {
		c.mine, c.answered = mine, asked
	}

	return c.yours(asked)
}

// authorCache is who the forge says a post would come from, kept from its first
// answer for the stream, GET /api/messaging and the announcement alike: the
// forge connection it comes through is kept once made, so the answer does not
// change while the server runs, and asking every frame spent a forge request
// per open page each interval. A read that fails is held with its error until
// the forge's interval has passed, so a forge that cannot say is asked once an
// interval too. One ask runs at a time, outside the lock: a frame that finds
// one under way is served the failure held at once, or, with none held, waits
// for that ask.
type authorCache struct {
	mu    sync.Mutex
	name  string
	known bool
	// failed is why the last ask did not answer, made at readAt, or nil.
	failed error
	readAt time.Time
	// reading is the ask under way, closed once it lands, or nil with none.
	reading chan struct{}
}

// cachedAuthor is who a post would come from: the forge's kept answer once it
// has given one, or why it could not say while that is held, else a fresh ask.
func (s *server) cachedAuthor() (string, error) {
	return sharedRead(
		func() turn[string] { return s.author.serve(s.now(), s.forgeInterval()) },
		func(int) (string, error) {
			name, err := s.deps.Author()
			if err != nil {
				return s.author.land(s.now(), "", fmt.Errorf("reading the author: %w", err))
			}

			return s.author.land(s.now(), name, nil)
		},
	)
}

// serve answers the author once known, and a failure while it is fresh or an
// ask is under way; it hands back that ask to wait for when nothing is held,
// and otherwise starts the caller's own ask, which land ends.
func (c *authorCache) serve(now time.Time, interval time.Duration) turn[string] {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch {
	case c.known:
		return turn[string]{answered: true, value: c.name}
	case c.failed != nil && (c.reading != nil || now.Sub(c.readAt) < interval):
		return turn[string]{answered: true, failed: c.failed}
	case c.reading != nil:
		return turn[string]{landing: c.reading}
	}

	c.reading = make(chan struct{})

	return turn[string]{}
}

// land holds what the ask made at now answered, or why it could not, and lets
// go every frame waiting on it. It answers what the frame shows.
func (c *authorCache) land(now time.Time, name string, err error) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.reading)
	c.reading = nil
	c.name, c.known, c.failed, c.readAt = name, err == nil, err, now

	return name, err
}

// scopeCache is the store's last commit scope, read the first time a frame
// asks — opening the store's database on every frame is the cost it avoids —
// and read again once after a commit here records one, so it holds what the
// store kept: nothing, when the store is off. Its lock is its own, since the
// streams read it while a commit clears it.
type scopeCache struct {
	mu    sync.Mutex
	read  bool
	value string
	found bool
}
