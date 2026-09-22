// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"bytes"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

const instance = "a1b2c3"

func TestACachedIssueListRoundTrips(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	payload := []byte(`[{"key":"PROJ-1"}]`)

	// Act
	err := kept.CacheIssues(t.Context(), instance, "assigned", payload, theTime())
	if err != nil {
		t.Fatalf("CacheIssues returned %v, want nil", err)
	}

	got, found, err := kept.CachedIssues(t.Context(), instance, "assigned")

	// Assert
	if err != nil || !found || !bytes.Equal(got, payload) {
		t.Errorf("CachedIssues = %q, %v, %v; want the cached payload", got, found, err)
	}
}

func TestTheCacheIsKeptPerInstanceAndView(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	_ = kept.CacheIssues(t.Context(), instance, "assigned", []byte("one"), theTime())

	// Act
	_, otherView, _ := kept.CachedIssues(t.Context(), instance, "reported")
	_, otherInstance, _ := kept.CachedIssues(t.Context(), "different", "assigned")

	// Assert
	if otherView || otherInstance {
		t.Error("the cache leaked across a view or an instance")
	}
}

func TestADisabledStoreCachesNoIssues(t *testing.T) {
	t.Parallel()

	// Arrange
	off := store.New(t.TempDir(), true)

	// Act
	err := off.CacheIssues(t.Context(), instance, "assigned", []byte("one"), theTime())
	_, found, _ := off.CachedIssues(t.Context(), instance, "assigned")

	// Assert
	if err != nil || found {
		t.Errorf("a disabled store cached issues (err %v, found %v); want it to no-op", err, found)
	}
}
