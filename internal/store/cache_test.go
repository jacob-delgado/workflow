// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

import (
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

const instance = "a1b2c3"

// issue is a cached issue named by its key; the other fields are fixed.
func issue(key string) store.CachedIssue {
	return store.CachedIssue{
		Key: key, Summary: key + " summary", Status: "To Do",
		StatusCategory: "new", Type: "Task", Priority: "High",
	}
}

func TestACachedIssueListRoundTripsInOrder(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	list := []store.CachedIssue{issue("PROJ-1"), issue("PROJ-2"), issue("PROJ-3")}

	// Act
	err := kept.CacheIssues(t.Context(), instance, "assigned", list, theTime())
	if err != nil {
		t.Fatalf("CacheIssues returned %v, want nil", err)
	}

	got, found, err := kept.CachedIssues(t.Context(), instance, "assigned")

	// Assert
	if err != nil || !found || !slices.Equal(got, list) {
		t.Errorf("CachedIssues = %v, %v, %v; want the cached list in order", got, found, err)
	}
}

func TestReCachingReplacesTheList(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	_ = kept.CacheIssues(t.Context(), instance, "assigned",
		[]store.CachedIssue{issue("PROJ-1"), issue("PROJ-2")}, theTime())

	// Act
	_ = kept.CacheIssues(t.Context(), instance, "assigned", []store.CachedIssue{issue("PROJ-9")}, theTime())
	got, _, _ := kept.CachedIssues(t.Context(), instance, "assigned")

	// Assert
	if !slices.Equal(got, []store.CachedIssue{issue("PROJ-9")}) {
		t.Errorf("CachedIssues = %v, want only the re-cached list, not appended", got)
	}
}

func TestAViewCachedEmptyIsFoundButEmpty(t *testing.T) {
	t.Parallel()

	// Arrange
	// A view that returned no issues is still recorded, so it opens as "no issues"
	// rather than loading again.
	kept := store.New(t.TempDir(), false)
	_ = kept.CacheIssues(t.Context(), instance, "assigned", nil, theTime())

	// Act
	got, found, err := kept.CachedIssues(t.Context(), instance, "assigned")

	// Assert
	if err != nil || !found || len(got) != 0 {
		t.Errorf("CachedIssues = %v, %v, %v; want found with no issues", got, found, err)
	}
}

func TestTheCacheIsKeptPerInstanceAndView(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	_ = kept.CacheIssues(t.Context(), instance, "assigned", []store.CachedIssue{issue("PROJ-1")}, theTime())

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
	err := off.CacheIssues(t.Context(), instance, "assigned", []store.CachedIssue{issue("PROJ-1")}, theTime())
	_, found, _ := off.CachedIssues(t.Context(), instance, "assigned")

	// Assert
	if err != nil || found {
		t.Errorf("a disabled store cached issues (err %v, found %v); want it to no-op", err, found)
	}
}
