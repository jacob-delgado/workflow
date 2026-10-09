// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// The kept database lives outside the process as the cache does, so these
// tests hand each kept read and write what it may find there instead of what
// it made: a path it cannot create, a table gone, a table of the wrong shape,
// a table whose trigger refuses a write, and a row that fails as it is read.
// Each is built from content, as failure_test.go's cases are.

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

func favorites(ctx context.Context, kept store.Store) (bool, error) {
	got, err := kept.Favorites(ctx)

	return len(got) > 0, err
}

func favorAPI(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.Favor(ctx, apiDir, theTime())
}

func unfavorAPI(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.Unfavor(ctx, apiDir)
}

func ownerLinks(ctx context.Context, kept store.Store) (bool, error) {
	links, err := kept.OwnerLinks(ctx, forgeHost, workspaceA)

	return len(links) > 0, err
}

func linkAnaOnSlack(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.LinkOwner(ctx, forgeHost, workspaceA, decided(anaOwner, ana()), theTime())
}

func linkAnaOffSlack(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.LinkOwner(ctx, forgeHost, workspaceA, decided(anaOwner, nil), theTime())
}

func forgetAna(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.ForgetOwner(ctx, forgeHost, workspaceA, anaOwner)
}

func repoGroups(ctx context.Context, kept store.Store) (bool, error) {
	groups, err := kept.RepoGroups(ctx, repo, workspaceA)

	return len(groups) > 0, err
}

func listAPIGroup(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.SetRepoGroups(ctx, repo, workspaceA, []store.SlackTarget{apiGroup()}, theTime())
}

func listNoGroup(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.SetRepoGroups(ctx, repo, workspaceA, nil, theTime())
}

func lastGroups(ctx context.Context, kept store.Store) (bool, error) {
	ids, chosen, err := kept.LastGroups(ctx, repo, workspaceA)

	return chosen || len(ids) > 0, err
}

func chooseAPIGroup(ctx context.Context, kept store.Store) (bool, error) {
	return false, kept.RecordGroups(ctx, repo, workspaceA, []string{apiID}, theTime())
}

// keptReads is every kept read, by the method it calls.
func keptReads() map[string]storeCall {
	return map[string]storeCall{
		"Favorites": favorites, "OwnerLinks": ownerLinks, "RepoGroups": repoGroups, "LastGroups": lastGroups,
	}
}

// eachKeptCall is every kept read and write, by the method it calls.
func eachKeptCall() map[string]storeCall {
	calls := keptReads()
	maps.Copy(calls, map[string]storeCall{
		"Favor": favorAPI, "Unfavor": unfavorAPI, "LinkOwner": linkAnaOnSlack, "ForgetOwner": forgetAna,
		"SetRepoGroups": listAPIGroup, "RecordGroups": chooseAPIGroup,
	})

	return calls
}

// readingGroups is the store's own words for reading a repository's groups.
const readingGroups = "reading the repository's groups"

// tamperKept runs each statement against the kept database directly, as
// another program would, in order on one connection.
func tamperKept(t *testing.T, dir string, statements ...statement) {
	t.Helper()

	database := openKeptFile(t, dir)

	for _, each := range statements {
		_, err := database.ExecContext(t.Context(), each.query, each.args...)
		if err != nil {
			t.Fatalf("tampering with %q: %v", each.query, err)
		}
	}
}

// refusing is a trigger that refuses every change of kind to table, as a
// table a file was found with may.
func refusing(kind, table string) statement {
	return statement{
		query: `CREATE TRIGGER refuse_` + strings.ToLower(kind) + `_` + table + ` BEFORE ` + kind + ` ON ` + table +
			` BEGIN SELECT RAISE(ABORT, 'refused'); END`,
	}
}

// gone drops table from the kept database.
func gone(table string) statement {
	return statement{query: `DROP TABLE ` + table}
}

// made makes the kept database and its schema, holding nothing.
func made(t *testing.T, kept store.Store) {
	t.Helper()

	err := kept.Unfavor(t.Context(), apiDir)
	if err != nil {
		t.Fatalf("making the kept database: %v", err)
	}
}

// listedAndChosen lists the api group for the repository and chooses it.
func listedAndChosen(t *testing.T, kept store.Store) {
	t.Helper()

	setGroups(t, kept, workspaceA, apiGroup())
	chooseGroups(t, kept, workspaceA, apiID)
}

// listedAPI lists the api group for the repository.
func listedAPI(t *testing.T, kept store.Store) {
	t.Helper()

	setGroups(t, kept, workspaceA, apiGroup())
}

func TestEveryKeptMethodReportsADirectoryItCannotCreate(t *testing.T) {
	t.Parallel()

	for name, call := range eachKeptCall() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A regular file where the store directory's parent should be: no
			// directory can be made inside a file, whoever asks.
			parent := filepath.Join(t.TempDir(), "not-a-directory")

			err := os.WriteFile(parent, []byte("a file"), 0o600)
			if err != nil {
				t.Fatal(err)
			}

			kept := store.New(filepath.Join(parent, "store"), false)

			// Act
			readBack, err := call(t.Context(), kept)

			// Assert
			if !errors.Is(err, syscall.ENOTDIR) || readBack {
				t.Errorf("%s = %v, read back %v; want the directory it could not create reported and nothing read",
					name, err, readBack)
			}
		})
	}
}

// tamperedKept is one kept table found other than the store made it: what
// the store held first, what was done to the file, the call that meets it,
// and the store's own words for the statement that does.
type tamperedKept struct {
	before func(t *testing.T, kept store.Store)
	tamper []statement
	call   storeCall
	step   string
}

// favoriteTampers are the favorite directories' table found gone or of the
// wrong shape.
func favoriteTampers() map[string]tamperedKept {
	return map[string]tamperedKept{
		"favorites read from a table that is gone": {
			before: made, tamper: []statement{gone("favorite_dir")}, call: favorites,
			step: "reading the favorite directories",
		},
		"a favorite marked in a table that is gone": {
			before: made, tamper: []statement{gone("favorite_dir")}, call: favorAPI,
			step: "marking a favorite directory",
		},
		"a favorite forgotten from a table that is gone": {
			before: made, tamper: []statement{gone("favorite_dir")}, call: unfavorAPI,
			step: "forgetting a favorite directory",
		},
		"a favorite with no directory": {
			before: made,
			tamper: []statement{
				gone("favorite_dir"),
				table(`CREATE TABLE favorite_dir (dir TEXT, added_at TEXT)`),
				table(`INSERT INTO favorite_dir (added_at) VALUES ('2026-10-05T00:00:00Z')`),
			},
			call: favorites, step: "reading a favorite directory",
		},
	}
}

// ownerTampers are the owner tables, and the Slack entities they link to,
// found gone, of the wrong shape, or refusing a change.
func ownerTampers() map[string]tamperedKept {
	return map[string]tamperedKept{
		"owner links read from a table that is gone": {
			before: made, tamper: []statement{gone("owner_decision")}, call: ownerLinks,
			step: "reading the owner links",
		},
		"an owner link with no kind": {
			before: made,
			tamper: []statement{
				gone("owner_decision"),
				table(`CREATE TABLE owner_decision (forge_host TEXT, owner TEXT, decided_at TEXT, owner_kind TEXT)`),
				{query: `INSERT INTO owner_decision (forge_host, owner) VALUES (?, ?)`, args: []any{forgeHost, anaOwner}},
				{
					query: `INSERT INTO owner_not_on_slack (forge_host, owner, slack_team) VALUES (?, ?, ?)`,
					args:  []any{forgeHost, anaOwner, workspaceA},
				},
			},
			call: ownerLinks, step: "reading an owner link",
		},
		"an owner decided in a table that is gone": {
			before: made, tamper: []statement{gone("owner_decision")}, call: linkAnaOnSlack,
			step: "recording the owner decision",
		},
		"an owner's decision cleared from a table that is gone": {
			before: made, tamper: []statement{gone("owner_not_on_slack")}, call: linkAnaOnSlack,
			step: "clearing what was decided for the owner",
		},
		"an owner forgotten from a table that is gone": {
			before: made, tamper: []statement{gone("owner_not_on_slack")}, call: forgetAna,
			step: "clearing what was decided for the owner",
		},
		"an owner's decision dropped from a table that refuses it": {
			before: linkAna, tamper: []statement{refusing("DELETE", "owner_decision")}, call: forgetAna,
			step: "forgetting the owner",
		},
		"a Slack user kept in a table that refuses it": {
			before: made, tamper: []statement{refusing("INSERT", "slack_entity")}, call: linkAnaOnSlack,
			step: "keeping the Slack entity",
		},
		"a Slack user no one links to pruned from a table that refuses it": {
			before: linkAna, tamper: []statement{refusing("DELETE", "slack_entity")}, call: linkAnaOffSlack,
			step: "pruning the Slack entities",
		},
	}
}

// groupTampers are the repository group tables found gone, of the wrong
// shape, or refusing a change.
func groupTampers() map[string]tamperedKept {
	return map[string]tamperedKept{
		"repository groups read from a table that is gone": {
			before: made, tamper: []statement{gone("repo_group")}, call: repoGroups,
			step: readingGroups,
		},
		"a repository group with no label": {
			before: made,
			tamper: []statement{
				gone("slack_entity"),
				table(`CREATE TABLE slack_entity (slack_id TEXT, label TEXT, seen_at TEXT)`),
				{query: `INSERT INTO slack_entity (slack_id) VALUES (?)`, args: []any{apiID}},
				{
					query: `INSERT INTO repo_group (repo, slack_team, slack_id, added_at) VALUES (?, ?, ?, ?)`,
					args:  []any{repo, workspaceA, apiID, "2026-10-05T00:00:00Z"},
				},
			},
			call: repoGroups, step: "reading a repository group",
		},
		"repository groups set in a table that is gone": {
			before: made, tamper: []statement{gone("repo_group")}, call: listAPIGroup,
			step: readingGroups,
		},
		"a repository group dropped from a table that refuses it": {
			before: listedAPI, tamper: []statement{refusing("DELETE", "repo_group")}, call: listNoGroup,
			step: "dropping a repository group",
		},
		"a group kept in a Slack table that refuses it": {
			before: made, tamper: []statement{refusing("INSERT", "slack_entity")}, call: listAPIGroup,
			step: "keeping the Slack entity",
		},
		"a repository group listed in a table that refuses it": {
			before: made, tamper: []statement{refusing("INSERT", "repo_group")}, call: listAPIGroup,
			step: "listing a repository group",
		},
	}
}

// choiceTampers are the tables of a repository's last choice of groups
// found gone, of the wrong shape, or refusing a change.
func choiceTampers() map[string]tamperedKept {
	return map[string]tamperedKept{
		"the last choice read from a table that is gone": {
			before: made, tamper: []statement{gone("repo_choice")}, call: lastGroups,
			step: "reading the last choice of groups",
		},
		"the last choice's groups read from a table that is gone": {
			before: listedAndChosen, tamper: []statement{gone("repo_choice_group")}, call: lastGroups,
			step: "reading the last choice of groups",
		},
		"a chosen group with no ID": {
			before: listedAndChosen,
			tamper: []statement{
				gone("repo_choice_group"),
				table(`CREATE TABLE repo_choice_group (repo TEXT, slack_team TEXT, slack_id TEXT)`),
				{query: `INSERT INTO repo_choice_group (repo, slack_team) VALUES (?, ?)`, args: []any{repo, workspaceA}},
			},
			call: lastGroups, step: "reading a Slack ID",
		},
		"a choice recorded in a table that is gone": {
			before: made, tamper: []statement{gone("repo_choice")}, call: chooseAPIGroup,
			step: "recording the choice of groups",
		},
		"a choice cleared from a table that is gone": {
			before: made, tamper: []statement{gone("repo_choice_group")}, call: chooseAPIGroup,
			step: "clearing the last choice of groups",
		},
		"a choice made among groups with no ID": {
			// The choice's own table names this one as its parent, so it keeps
			// its key; SQLite lets an old table's key hold a NULL.
			before: made,
			tamper: []statement{
				gone("repo_group"),
				table(`CREATE TABLE repo_group (
					repo TEXT, slack_team TEXT, slack_id TEXT, added_at TEXT, PRIMARY KEY (repo, slack_team, slack_id)
				)`),
				{query: `INSERT INTO repo_group (repo, slack_team) VALUES (?, ?)`, args: []any{repo, workspaceA}},
			},
			call: chooseAPIGroup, step: "reading a Slack ID",
		},
		"a chosen group recorded in a table that refuses it": {
			before: listedAPI, tamper: []statement{refusing("INSERT", "repo_choice_group")}, call: chooseAPIGroup,
			step: "recording a chosen group",
		},
	}
}

func TestATamperedKeptTableIsReportedNotTrusted(t *testing.T) {
	t.Parallel()

	cases := map[string]tamperedKept{}

	for _, each := range []map[string]tamperedKept{favoriteTampers(), ownerTampers(), groupTampers(), choiceTampers()} {
		maps.Copy(cases, each)
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			kept := store.New(dir, false)
			testCase.before(t, kept)
			tamperKept(t, dir, testCase.tamper...)

			// Act
			readBack, err := testCase.call(t.Context(), kept)

			// Assert
			if err == nil || !strings.Contains(err.Error(), testCase.step) || readBack {
				t.Errorf("err = %v, read back %v; want the tampered table reported by %q and nothing read",
					err, readBack, testCase.step)
			}
		})
	}
}
