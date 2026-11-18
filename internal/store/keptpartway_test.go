// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// A kept read can fail after it has begun, on a row that fails as it is read
// after others were; a dry run's read can meet a kept file that is no
// database; and a kept file at no version can hold a table another program
// left there. Each is built from content the store finds there, as
// keptfailure_test.go's cases are.

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

// listedAndChosenBoth lists the api and pod groups for the repository and
// chooses both.
func listedAndChosenBoth(t *testing.T, kept store.Store) {
	t.Helper()

	setGroups(t, kept, workspaceA, apiGroup(), *podGroup())
	chooseGroups(t, kept, workspaceA, apiID, podID)
}

// listedBoth lists the api and pod groups for the repository.
func listedBoth(t *testing.T, kept store.Store) {
	t.Helper()

	setGroups(t, kept, workspaceA, apiGroup(), *podGroup())
}

func TestAKeptReadThatFailsPartwayReturnsNothing(t *testing.T) {
	t.Parallel()

	// SQLite reads a query's first row as the query starts, so each view here
	// answers one good row and fails on its second, while the read is under
	// way. Each reads its rows in the order its query asks for, so neither is
	// read before the first is answered.
	cases := map[string]tamperedKept{
		"favorites": {
			before: made,
			tamper: []statement{
				dropped("favorite_dir"),
				table(`CREATE TABLE marked (dir TEXT PRIMARY KEY, number INTEGER) WITHOUT ROWID`),
				{query: `INSERT INTO marked (dir, number) VALUES (?, 1), (?, ?)`, args: []any{apiDir, webDir, smallestInteger}},
				table(`CREATE VIEW favorite_dir AS SELECT dir,
					CASE WHEN abs(number) > 0 THEN '2026-10-05T00:00:00Z' END AS added_at FROM marked`),
			},
			call: favorites, step: "reading the favorite directories",
		},
		"owner links": {
			before: made,
			tamper: []statement{
				dropped("owner_decision"),
				table(`CREATE TABLE decided (
					forge_host TEXT, owner TEXT, number INTEGER, PRIMARY KEY (forge_host, owner)
				) WITHOUT ROWID`),
				{
					query: `INSERT INTO decided (forge_host, owner, number) VALUES (?, ?, 1), (?, ?, ?)`,
					args:  []any{forgeHost, anaOwner, forgeHost, boOwner, smallestInteger},
				},
				table(`CREATE VIEW owner_decision AS SELECT forge_host, owner, '2026-10-05T00:00:00Z' AS decided_at,
					CASE WHEN abs(number) > 0 THEN 'user' END AS owner_kind FROM decided`),
				{
					query: `INSERT INTO owner_not_on_slack (forge_host, owner, slack_team) VALUES (?, ?, ?), (?, ?, ?)`,
					args:  []any{forgeHost, anaOwner, workspaceA, forgeHost, boOwner, workspaceA},
				},
			},
			call: ownerLinks, step: "reading the owner links",
		},
		"repository groups": {
			// The groups are read by label, in the order the entities' own key
			// keeps them, each looked up among the listed by its ID. Every open
			// makes the store's index on repo_group, which a view cannot take,
			// so the view stands in for the Slack entities instead.
			before: listedBoth,
			tamper: []statement{
				dropped("slack_entity"),
				table(`CREATE TABLE entity (
					label TEXT, slack_id TEXT, number INTEGER, PRIMARY KEY (label, slack_id)
				) WITHOUT ROWID`),
				{
					query: `INSERT INTO entity (label, slack_id, number) VALUES (?, ?, 1), (?, ?, ?)`,
					args:  []any{apiGroup().Label, apiID, podGroup().Label, podID, smallestInteger},
				},
				table(`CREATE VIEW slack_entity AS
					SELECT label, slack_id, '' AS seen_at FROM entity WHERE abs(number) > 0`),
			},
			call: repoGroups, step: readingGroups,
		},
		"the groups of the last choice": {
			before: listedAndChosenBoth,
			tamper: []statement{
				dropped("repo_choice_group"),
				table(`CREATE TABLE chosen (
					repo TEXT, slack_team TEXT, slack_id TEXT, number INTEGER, PRIMARY KEY (repo, slack_team, slack_id)
				) WITHOUT ROWID`),
				{
					query: `INSERT INTO chosen (repo, slack_team, slack_id, number) VALUES (?, ?, ?, 1), (?, ?, ?, ?)`,
					args:  []any{repo, workspaceA, apiID, repo, workspaceA, podID, smallestInteger},
				},
				table(`CREATE VIEW repo_choice_group AS
					SELECT repo, slack_team, slack_id FROM chosen WHERE abs(number) > 0`),
			},
			call: lastGroups, step: "reading the Slack IDs",
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			kept := store.New(dir, store.ModeWrites)
			testCase.before(t, kept)
			tamperKept(t, dir, testCase.tamper...)

			// Act
			readBack, err := testCase.call(t.Context(), kept)

			// Assert
			if err == nil || !strings.Contains(err.Error(), testCase.step) || readBack {
				t.Errorf("err = %v, read back %v; want the failed read reported by %q and the rows before it dropped",
					err, readBack, testCase.step)
			}
		})
	}
}

func TestAReadOnlyStoreReportsAKeptFileThatIsNotADatabase(t *testing.T) {
	t.Parallel()

	for name, call := range keptReads() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			notADatabase := []byte(strings.Repeat("this is not a database\n", 45))

			err := os.WriteFile(keptPath(dir), notADatabase, 0o600)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			readBack, err := call(t.Context(), store.New(dir, store.ModeWrites).ReadOnly())

			// Assert
			if err == nil || !strings.Contains(err.Error(), "reading the store schema version") || readBack {
				t.Errorf("%s = %v, read back %v; want the file reported and nothing read", name, err, readBack)
			}

			left, readErr := os.ReadFile(keptPath(dir))
			if readErr != nil || !bytes.Equal(left, notADatabase) {
				t.Errorf("the file now holds %d bytes (err %v), want it left as it was", len(left), readErr)
			}
		})
	}
}

func TestAKeptFileAtNoVersionHoldingATableIsMadeAgain(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file at no version that holds a table is not a fresh one, so it is
	// another build's or another program's, and is made again like any other
	// file at another version.
	dir := t.TempDir()
	execKept(t, dir, `CREATE TABLE favorite_dir (other TEXT)`)
	kept := store.New(dir, store.ModeWrites)

	// Act
	err := kept.Favor(t.Context(), apiDir, theTime())

	// Assert
	favorites, readErr := kept.Favorites(t.Context())

	want := []store.Favorite{{Dir: apiDir, Added: theTime()}}
	if err != nil || readErr != nil || !slices.Equal(favorites, want) {
		t.Errorf("after Favor = %v: Favorites = %+v, %v; want %+v", err, favorites, readErr, want)
	}

	if got := keptUserVersion(t, dir); got != keptVersion {
		t.Errorf("the file is at version %d, want it made again at %d", got, keptVersion)
	}
}
