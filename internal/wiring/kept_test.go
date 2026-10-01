// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The kept associations are keyed by the forge host alone — an owner is the
// same person in every repository on it — and a repository's groups by the
// repository, both parsed from the remote so no credential reaches the file.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// anaOwner is the forge owner the tests link to the Slack user Ana.
const anaOwner = "ana"

// credentialedRemote is an HTTPS remote carrying a credential in its userinfo.
const credentialedRemote = "https://u:tok@github.com/a/b.git"

// ana is the Slack user the tests link a forge owner to.
func ana() *loop.SlackTarget {
	return &loop.SlackTarget{ID: "U012ABC", Label: "Ana Lima"}
}

// podGroup is a Slack user group a repository may tag.
func podGroup() loop.SlackTarget {
	return loop.SlackTarget{ID: "S0POD123", Label: "control-plane-pod"}
}

// isolatedStoreDir points the store at a fresh home and returns its directory.
func isolatedStoreDir(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", "")

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("resolving the store directory: %v", err)
	}

	return dir
}

func TestAnOwnerLinkIsKeyedByTheForgeHostAlone(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)

	// Act
	err := deps.Store.LinkOwner(anaOwner, ana())
	// Assert
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := store.New(dir, false).OwnerLinks(t.Context(), "github.com")
	if err != nil || len(links) != 1 || links[0].Owner != anaOwner {
		t.Errorf("the store's links on github.com = %+v, %v; want ana's", links, err)
	}

	if storeHoldsToken(t, dir, "tok") {
		t.Error("the store holds the remote's credential \"tok\"")
	}
}

func TestAnOwnerLinkIsSharedByEveryRepositoryOnTheHost(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)
	first := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)
	second := wired(t, config.Default(),
		wiring.Workspace{Root: t.TempDir(), Remote: "git@GitHub.com:other/repo.git"}, nil)

	err := first.Store.LinkOwner(anaOwner, ana())
	if err != nil {
		t.Fatalf("linking ana: %v", err)
	}

	// Act
	links, err := second.Store.OwnerLinks()

	// Assert
	want := []loop.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *ana()}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks in another repository on the host = %+v, %v; want %+v", links, err, want)
	}
}

func TestAForgottenOwnerIsUndecidedThroughTheSeams(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)

	err := deps.Store.LinkOwner("dan", nil)
	if err != nil {
		t.Fatalf("deciding dan: %v", err)
	}

	// Act
	err = deps.Store.ForgetOwner("dan")
	// Assert
	if err != nil {
		t.Fatalf("ForgetOwner returned %v, want nil", err)
	}

	if links, _ := deps.Store.OwnerLinks(); len(links) != 0 {
		t.Errorf("OwnerLinks after forgetting dan = %+v, want none", links)
	}
}

func TestALinkOfTheWrongShapeIsRefusedThroughTheSeams(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)
	group := podGroup()

	// Act
	err := deps.Store.LinkOwner(anaOwner, &group)

	// Assert
	if !errors.Is(err, store.ErrInvalidSlackID) {
		t.Errorf("linking a user owner to a group = %v, want ErrInvalidSlackID", err)
	}
}

func TestTheGroupsOfARepositoryAreKeyedByTheRepository(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)

	// Act
	err := deps.Store.SetRepoGroups([]loop.SlackTarget{podGroup()})
	// Assert
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	groups, err := store.New(dir, false).RepoGroups(t.Context(), "github.com/a/b")
	if err != nil || len(groups) != 1 || groups[0].ID != podGroup().ID {
		t.Errorf("the store's groups for github.com/a/b = %+v, %v; want the pod's", groups, err)
	}

	if storeHoldsToken(t, dir, "tok") {
		t.Error("the store holds the remote's credential \"tok\"")
	}
}

func TestTheLastChoiceOfGroupsRoundTripsThroughTheSeams(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)

	err := deps.Store.SetRepoGroups([]loop.SlackTarget{podGroup()})
	if err != nil {
		t.Fatalf("listing the group: %v", err)
	}

	// Act
	err = deps.Store.RecordGroups([]string{podGroup().ID})
	if err != nil {
		t.Fatalf("RecordGroups returned %v, want nil", err)
	}

	ids, chosen := deps.Store.LastGroups()

	// Assert
	if !chosen || !slices.Equal(ids, []string{podGroup().ID}) {
		t.Errorf("LastGroups = %v, %v; want the pod chosen", ids, chosen)
	}
}

func TestADisabledStoreOffersNoKeptAssociations(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)

	cfg := config.Default()
	cfg.Store.Disabled = true

	// Act
	deps := wired(t, cfg, wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote}, nil)

	// Assert
	kept := deps.Store
	if kept.OwnerLinks != nil || kept.LinkOwner != nil || kept.ForgetOwner != nil || kept.RepoGroups != nil ||
		kept.SetRepoGroups != nil || kept.LastGroups != nil || kept.RecordGroups != nil {
		t.Error("a disabled store bound kept associations, so a surface would offer what saves nothing")
	}
}

func TestNoForgeHostOffersNoOwnerLinks(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)

	// Act
	deps := wired(t, config.Default(), wiring.Workspace{Root: t.TempDir(), Remote: ""}, nil)

	// Assert
	kept := deps.Store
	if kept.OwnerLinks != nil || kept.LinkOwner != nil || kept.ForgetOwner != nil {
		t.Error("owner links were bound with no forge host to key them by, so a link would save nothing")
	}

	if kept.RepoGroups == nil || kept.SetRepoGroups == nil {
		t.Error("a repository's groups were not bound, though the repository's root keys them")
	}
}

func TestADryRunNeverMakesTheKeptFile(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	readOnly := wiring.ReadOnlyStore(t.Context(), config.Default(),
		wiring.Workspace{Root: t.TempDir(), Remote: credentialedRemote})

	// Act
	linkErr := readOnly.LinkOwner(anaOwner, ana())
	groupsErr := readOnly.SetRepoGroups([]loop.SlackTarget{podGroup()})

	// Assert
	if linkErr != nil || groupsErr != nil {
		t.Errorf("a dry run's writes = %v, %v; want them to do nothing, without error", linkErr, groupsErr)
	}

	_, statErr := os.Stat(filepath.Join(dir, "kept.db"))
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a dry run made kept.db (stat: %v), want nothing on disk", statErr)
	}
}
