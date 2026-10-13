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
)

// anaOwner is the forge owner the tests link to the Slack user Ana.
const anaOwner = "ana"

// workspace is the Slack workspace the tests keep links in.
const workspace = "T0EXAMPLE"

// credentialedRemote is an HTTPS remote carrying a credential in its userinfo.
const credentialedRemote = "https://u:tok@github.com/a/b.git"

// ana is the Slack user the tests link a forge owner to.
func ana() *loop.SlackTarget {
	return &loop.SlackTarget{ID: "U012ABC", Label: "Ana Lima"}
}

// anaLinked is the decision that the forge owner ana is the Slack user Ana.
func anaLinked() loop.OwnerLink {
	return loop.OwnerLink{Owner: anaOwner, Team: false, OnSlack: true, Slack: *ana()}
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
	deps := wired(t, config.Default(), onOrigin(t.TempDir(), credentialedRemote), nil)

	// Act
	err := deps.Store.LinkOwner(t.Context(), workspace, anaLinked())
	// Assert
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := store.New(dir, false).OwnerLinks(t.Context(), "github.com", workspace)
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
	first := wired(t, config.Default(), onOrigin(t.TempDir(), credentialedRemote), nil)
	second := wired(t, config.Default(),
		onOrigin(t.TempDir(), "git@GitHub.com:other/repo.git"), nil)

	err := first.Store.LinkOwner(t.Context(), workspace, anaLinked())
	if err != nil {
		t.Fatalf("linking ana: %v", err)
	}

	// Act
	links, err := second.Store.OwnerLinks(t.Context(), workspace)

	// Assert
	want := []loop.OwnerLink{{Owner: anaOwner, OnSlack: true, Slack: *ana()}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks in another repository on the host = %+v, %v; want %+v", links, err, want)
	}
}

func TestAForgottenOwnerIsUndecidedThroughTheSeams(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)
	deps := wired(t, config.Default(), onOrigin(t.TempDir(), credentialedRemote), nil)

	notOnSlack := loop.OwnerLink{Owner: "dan", Team: false, OnSlack: false, Slack: loop.SlackTarget{}}

	err := deps.Store.LinkOwner(t.Context(), workspace, notOnSlack)
	if err != nil {
		t.Fatalf("deciding dan: %v", err)
	}

	// Act
	err = deps.Store.ForgetOwner(t.Context(), workspace, "dan")
	// Assert
	if err != nil {
		t.Fatalf("ForgetOwner returned %v, want nil", err)
	}

	if links, _ := deps.Store.OwnerLinks(t.Context(), workspace); len(links) != 0 {
		t.Errorf("OwnerLinks after forgetting dan = %+v, want none", links)
	}
}

func TestALinkOfTheWrongShapeIsRefusedThroughTheSeams(t *testing.T) {
	// Arrange
	isolatedStoreDir(t)
	deps := wired(t, config.Default(), onOrigin(t.TempDir(), credentialedRemote), nil)
	link := loop.OwnerLink{Owner: anaOwner, Team: false, OnSlack: true, Slack: podGroup()}

	// Act
	err := deps.Store.LinkOwner(t.Context(), workspace, link)

	// Assert
	if !errors.Is(err, store.ErrInvalidSlackID) {
		t.Errorf("linking a user owner to a group = %v, want ErrInvalidSlackID", err)
	}
}

func TestTheGroupsOfARepositoryAreKeyedByTheRepository(t *testing.T) {
	// Arrange
	dir := isolatedStoreDir(t)
	deps := wired(t, config.Default(), onOrigin(t.TempDir(), credentialedRemote), nil)

	// Act
	err := deps.Store.SetRepoGroups(t.Context(), workspace, []loop.SlackTarget{podGroup()})
	// Assert
	if err != nil {
		t.Fatalf("SetRepoGroups returned %v, want nil", err)
	}

	groups, err := store.New(dir, false).RepoGroups(t.Context(), "github.com/a/b", workspace)
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
	deps := wired(t, config.Default(), onOrigin(t.TempDir(), credentialedRemote), nil)

	err := deps.Store.SetRepoGroups(t.Context(), workspace, []loop.SlackTarget{podGroup()})
	if err != nil {
		t.Fatalf("listing the group: %v", err)
	}

	// Act
	err = deps.Store.RecordGroups(t.Context(), workspace, []string{podGroup().ID})
	if err != nil {
		t.Fatalf("RecordGroups returned %v, want nil", err)
	}

	ids, chosen := deps.Store.LastGroups(t.Context(), workspace)

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
	deps := wired(t, cfg, onOrigin(t.TempDir(), credentialedRemote), nil)

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
	deps := wired(t, config.Default(), onOrigin(t.TempDir(), ""), nil)

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
	readOnly := processEnvironment().ReadOnlyStore(config.Default(),
		onOrigin(t.TempDir(), credentialedRemote))

	// Act
	linkErr := readOnly.LinkOwner(t.Context(), workspace, anaLinked())
	groupsErr := readOnly.SetRepoGroups(t.Context(), workspace, []loop.SlackTarget{podGroup()})

	// Assert
	if linkErr != nil || groupsErr != nil {
		t.Errorf("a dry run's writes = %v, %v; want them to do nothing, without error", linkErr, groupsErr)
	}

	_, statErr := os.Stat(filepath.Join(dir, "kept.db"))
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("a dry run made kept.db (stat: %v), want nothing on disk", statErr)
	}
}
