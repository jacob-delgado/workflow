// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package lifetime_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/lifetime"
)

// errStopped is the cause a lifetime ends with in these tests.
var errStopped = errors.New("the server stopped")

// valueKey keys the value a detached context keeps.
type valueKey struct{}

func TestADetachedContextGoesOnWhenItsAskerLeaves(t *testing.T) {
	t.Parallel()

	// Arrange
	asker, leave := context.WithCancel(lifetime.Within(t.Context(), t.Context()))
	detached, release := lifetime.Detached(asker)
	t.Cleanup(release)

	// Act
	leave()

	// Assert
	err := detached.Err()
	if err != nil {
		t.Errorf("the detached context ended with %v once its asker left, want it to go on", err)
	}
}

func TestADetachedContextEndsWithTheLifetimeItsAskerCarries(t *testing.T) {
	t.Parallel()

	// Arrange
	serving, stop := context.WithCancelCause(t.Context())
	detached, release := lifetime.Detached(lifetime.Within(t.Context(), serving))
	t.Cleanup(release)

	// Act
	stop(errStopped)

	// Assert
	<-detached.Done()

	cause := context.Cause(detached)
	if !errors.Is(cause, errStopped) {
		t.Errorf("the detached context ended with %v, want the lifetime's cause, %v", cause, errStopped)
	}
}

func TestADetachedContextWithNoLifetimeNeverEnds(t *testing.T) {
	t.Parallel()

	// Arrange
	asker, leave := context.WithCancel(t.Context())
	detached, release := lifetime.Detached(asker)
	t.Cleanup(release)

	// Act
	leave()

	// Assert
	err := detached.Err()
	if err != nil {
		t.Errorf("the detached context ended with %v, want it to go on with no lifetime to end with", err)
	}
}

func TestReleasingADetachedContextEndsIt(t *testing.T) {
	t.Parallel()

	// Arrange
	detached, release := lifetime.Detached(lifetime.Within(t.Context(), t.Context()))

	// Act
	release()

	// Assert
	err := detached.Err()
	if !errors.Is(err, context.Canceled) {
		t.Errorf("the released context = %v, want it %v", err, context.Canceled)
	}
}

func TestADetachedContextKeepsItsAskersValues(t *testing.T) {
	t.Parallel()

	// Arrange
	asker := context.WithValue(t.Context(), valueKey{}, "kept")

	// Act
	detached, release := lifetime.Detached(asker)
	t.Cleanup(release)

	// Assert
	if got := detached.Value(valueKey{}); got != "kept" {
		t.Errorf("the detached context's value = %v, want the asker's, kept", got)
	}
}
