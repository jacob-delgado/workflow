// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/armed"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/webserver"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

// stoppedClock is a clock that tells the time it was last moved to.
type stoppedClock struct {
	mu  sync.Mutex
	now time.Time
}

// tell is the time the clock tells.
func (c *stoppedClock) tell() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

// advance moves the clock on by by.
func (c *stoppedClock) advance(by time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.now = c.now.Add(by)
}

// noCIDeps is reviewDeps on a repository that reports no check on the pull
// request, served on clock (Q34).
func noCIDeps(clock *stoppedClock) webserver.Deps {
	deps := reviewDeps(&reviewWrites{})
	deps.Clock = clock.tell
	deps.Forge.CheckStatus = func(context.Context, forge.PullRequest, string) (forge.CI, error) {
		return forge.CI{State: forge.CINone}, nil
	}

	return deps
}

// secondCIReadFails is deps whose second read of CI fails, every other reporting no
// check: no time is counted across a read that failed.
func secondCIReadFails(deps webserver.Deps) webserver.Deps {
	var (
		reading sync.Mutex
		reads   int
	)

	deps.Forge.CheckStatus = func(context.Context, forge.PullRequest, string) (forge.CI, error) {
		reading.Lock()
		defer reading.Unlock()

		reads++
		if reads == 2 {
			return forge.CI{}, errCIUnread
		}

		return forge.CI{State: forge.CINone}, nil
	}

	return deps
}

func TestMergeUnderNoCICountsNoQuietAcrossAReadThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	clock := &stoppedClock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	served := webtest.Serve(t, secondCIReadFails(noCIDeps(clock)), config.Default())
	webtest.Get(t, served, mergeAt)
	clock.advance(time.Minute)
	webtest.Get(t, served, mergeAt)
	clock.advance(armed.NoCIPatience - time.Minute)

	// Act
	recorder := webtest.Get(t, served, mergeAt)

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Errorf("GET %s = %d, want 409: the ten minutes ran across a read that failed", mergeAt, recorder.Code)
	}
}

func TestTheReviewCountsNoQuietAcrossAReadThatFailed(t *testing.T) {
	t.Parallel()

	// Arrange
	clock := &stoppedClock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	served := webtest.Serve(t, secondCIReadFails(noCIDeps(clock)), config.Default())
	webtest.Get(t, served, "/api/review")
	clock.advance(time.Minute)
	webtest.Get(t, served, "/api/review")
	clock.advance(armed.NoCIPatience - time.Minute)

	// Act
	review := webtest.Decode[api.Review](t, webtest.Get(t, served, "/api/review"))

	// Assert
	if review.Ci == nil || review.Ci.NoCi {
		t.Errorf("review CI = %+v, want no_ci false: the ten minutes ran across a read that failed", review.Ci)
	}
}

func TestMergeUnderNoCIIsRefusedBeforeTheHeadWasQuietForTheWait(t *testing.T) {
	t.Parallel()

	// Arrange
	// Q34: a forge lists a pushed head's checks a moment after the push, so
	// none reported yet is not taken for no CI.
	clock := &stoppedClock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	served := webtest.Serve(t, noCIDeps(clock), config.Default())

	// Act
	recorder := webtest.Get(t, served, mergeAt)

	// Assert
	if recorder.Code != http.StatusConflict {
		t.Errorf("GET %s = %d, want 409 before ten minutes of no check", mergeAt, recorder.Code)
	}
}

func TestMergeUnderNoCIIsOfferedOnceTheHeadWasQuietForTheWait(t *testing.T) {
	t.Parallel()

	// Arrange
	clock := &stoppedClock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	served := webtest.Serve(t, noCIDeps(clock), config.Default())
	webtest.Get(t, served, mergeAt)
	clock.advance(armed.NoCIPatience)

	// Act
	recorder := webtest.Get(t, served, mergeAt)

	// Assert
	offer := webtest.Decode[api.MergeOffer](t, recorder)
	if offer.Pull.Number != 42 || len(offer.Methods) == 0 || offer.NoCiWaitMinutes != 10 {
		t.Errorf("offer = %+v, want #42 offered with its methods once no check was reported for the wait, "+
			"saying it waited 10 minutes", offer)
	}
}

func TestTheReviewSaysWhenTheHeadWasQuietForTheWait(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		after time.Duration
		want  bool
	}{
		"a moment too early":  {after: armed.NoCIPatience - time.Second, want: false},
		"as long as it waits": {after: armed.NoCIPatience, want: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			clock := &stoppedClock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
			served := webtest.Serve(t, noCIDeps(clock), config.Default())
			webtest.Get(t, served, "/api/review")
			clock.advance(tt.after)

			// Act
			review := webtest.Decode[api.Review](t, webtest.Get(t, served, "/api/review"))

			// Assert
			if review.Ci == nil || review.Ci.NoCi != tt.want {
				t.Errorf("review CI = %+v, want no_ci %t", review.Ci, tt.want)
			}
		})
	}
}
