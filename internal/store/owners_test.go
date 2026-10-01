// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// A forge owner is asked about once: whom they are on Slack, or that they are
// not on Slack. What is read back is validated, since the file is tamperable:
// a row of the wrong shape reads as never decided, so the owner is asked again.

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jacob-delgado/workflow/internal/store"
)

// podGroup is a Slack user group a team owner is linked to.
func podGroup() *store.SlackTarget {
	return &store.SlackTarget{ID: "S0POD123", Label: "control-plane-pod"}
}

// keptEntities counts the Slack users and groups the kept file holds.
func keptEntities(t *testing.T, dir string) int {
	t.Helper()

	var count int

	err := openKeptFile(t, dir).QueryRowContext(t.Context(), `SELECT COUNT(*) FROM slack_entity`).Scan(&count)
	if err != nil {
		t.Fatalf("counting the Slack entities: %v", err)
	}

	return count
}

func TestAnOwnerNotOnSlackIsRememberedAsDecided(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, "dan", nil, theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost)

	// Assert
	want := []store.OwnerLink{{Owner: "dan", OnSlack: false, Slack: store.SlackTarget{}}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks = %+v, %v; want %+v", links, err, want)
	}
}

func TestATeamOwnerLinksToAUserGroup(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, "acme/control-plane", podGroup(), theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost)

	// Assert
	want := []store.OwnerLink{{Owner: "acme/control-plane", OnSlack: true, Slack: *podGroup()}}
	if err != nil || !slices.Equal(links, want) {
		t.Errorf("OwnerLinks = %+v, %v; want %+v", links, err, want)
	}
}

func TestRelinkingAnOwnerReplacesTheDecision(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	err := kept.LinkOwner(t.Context(), forgeHost, anaOwner, nil, theTime())
	if err != nil {
		t.Fatalf("LinkOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost)

	// Assert
	if err != nil || len(links) != 1 || links[0].OnSlack {
		t.Errorf("OwnerLinks = %+v, %v; want ana alone, not on Slack", links, err)
	}
}

func TestOwnerLinksAreKeptPerForgeHost(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	links, err := kept.OwnerLinks(t.Context(), "gitlab.example.com")

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks on another host = %+v, %v; want none", links, err)
	}
}

func TestAForgottenOwnerIsUndecided(t *testing.T) {
	t.Parallel()

	// Arrange
	kept := store.New(t.TempDir(), false)
	linkAna(t, kept)

	// Act
	err := kept.ForgetOwner(t.Context(), forgeHost, anaOwner)
	if err != nil {
		t.Fatalf("ForgetOwner returned %v, want nil", err)
	}

	links, err := kept.OwnerLinks(t.Context(), forgeHost)

	// Assert
	if err != nil || len(links) != 0 {
		t.Errorf("OwnerLinks after forgetting ana = %+v, %v; want none", links, err)
	}
}

func TestASlackEntityNoOneLinksToIsPruned(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		change func(store.Store) error
		left   int
	}{
		"relinked to someone else": {
			change: func(kept store.Store) error {
				return kept.LinkOwner(t.Context(), forgeHost, anaOwner,
					&store.SlackTarget{ID: "U099XYZ", Label: "Ana L."}, theTime())
			},
			left: 1,
		},
		"marked not on Slack": {
			change: func(kept store.Store) error {
				return kept.LinkOwner(t.Context(), forgeHost, anaOwner, nil, theTime())
			},
			left: 0,
		},
		"forgotten": {
			change: func(kept store.Store) error { return kept.ForgetOwner(t.Context(), forgeHost, anaOwner) },
			left:   0,
		},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			kept := store.New(dir, false)
			linkAna(t, kept)

			// Act
			err := each.change(kept)
			// Assert
			if err != nil {
				t.Fatalf("changing ana's link: %v", err)
			}

			if got := keptEntities(t, dir); got != each.left {
				t.Errorf("the kept file holds %d Slack entities, want %d", got, each.left)
			}
		})
	}
}

func TestALinkOfTheWrongShapeIsRefused(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		owner  string
		target *store.SlackTarget
		want   error
	}{
		"a user owner linked to a group": {anaOwner, podGroup(), store.ErrInvalidSlackID},
		"a team owner linked to a user":  {"acme/pod", ana(), store.ErrInvalidSlackID},
		"a lowercase ID":                 {anaOwner, &store.SlackTarget{ID: "u012abc", Label: ""}, store.ErrInvalidSlackID},
		"an ID too short":                {anaOwner, &store.SlackTarget{ID: "U1", Label: ""}, store.ErrInvalidSlackID},
		"an owner of the wrong shape":    {"-ana", nil, store.ErrInvalidOwner},
		"an owner with a control":        {"an\x1ba", nil, store.ErrInvalidOwner},
		"an owner with an empty segment": {"acme//pod", podGroup(), store.ErrInvalidOwner},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			kept := store.New(t.TempDir(), false)

			// Act
			err := kept.LinkOwner(t.Context(), forgeHost, each.owner, each.target, theTime())

			// Assert
			if !errors.Is(err, each.want) {
				t.Errorf("LinkOwner = %v, want %v", err, each.want)
			}
		})
	}
}

func TestAStoredRowOfTheWrongShapeReadsAsUndecided(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a user owner linked to a group": `UPDATE owner_slack SET slack_id = 'S0POD123' WHERE owner = 'ana'`,
		"a malformed ID":                 `UPDATE owner_slack SET slack_id = 'X1' WHERE owner = 'ana'`,
		"a link to no Slack entity":      `DELETE FROM slack_entity WHERE slack_id = 'U012ABC'`,
		"an owner of the wrong shape":    `UPDATE owner_decision SET owner = 'an a' WHERE owner = 'ana'`,
	}

	for name, tamper := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			kept := store.New(dir, false)
			linkAna(t, kept)

			err := kept.LinkOwner(t.Context(), forgeHost, "acme/pod", podGroup(), theTime())
			if err != nil {
				t.Fatalf("linking the team: %v", err)
			}

			execKept(t, dir, `PRAGMA foreign_keys = OFF`)
			execKept(t, dir, tamper)

			// Act
			links, err := kept.OwnerLinks(t.Context(), forgeHost)

			// Assert
			want := []store.OwnerLink{{Owner: "acme/pod", OnSlack: true, Slack: *podGroup()}}
			if err != nil || !slices.Equal(links, want) {
				t.Errorf("OwnerLinks = %+v, %v; want only the untampered team %+v", links, err, want)
			}
		})
	}
}

func TestAStoredLabelIsSanitizedAndCapped(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	kept := store.New(dir, false)
	linkAna(t, kept)
	execKept(t, dir, `UPDATE slack_entity SET label = 'Ana'||char(27)||'[2J'||'`+strings.Repeat("x", 300)+`'`)

	// Act
	links, err := kept.OwnerLinks(t.Context(), forgeHost)

	// Assert
	if err != nil || len(links) != 1 {
		t.Fatalf("OwnerLinks = %+v, %v; want ana's link", links, err)
	}

	label := links[0].Slack.Label
	if strings.ContainsRune(label, '\x1b') || utf8.RuneCountInString(label) > 80 {
		t.Errorf("the label read back is %q (%d runes), want no escape and at most 80 runes",
			label, utf8.RuneCountInString(label))
	}
}
