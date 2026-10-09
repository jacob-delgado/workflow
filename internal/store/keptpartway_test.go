// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store_test

// A kept read can fail after it has begun, on a row that fails as it is read
// after others were; a dry run's read can meet a kept file that is no
// database; and a fresh kept file can hold a table another program left in
// the way of the store's schema. Each is built from content the store finds
// there, as keptfailure_test.go's cases are.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/store"
)

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
				gone("favorite_dir"),
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
				gone("owner_decision"),
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
			// keeps them, each looked up among the listed by its ID.
			before: made,
			tamper: []statement{
				gone("repo_group"),
				gone("slack_entity"),
				table(`CREATE TABLE slack_entity (
					label TEXT, slack_id TEXT, seen_at TEXT, PRIMARY KEY (label, slack_id)
				) WITHOUT ROWID`),
				{
					query: `INSERT INTO slack_entity (label, slack_id, seen_at) VALUES (?, ?, ''), (?, ?, '')`,
					args:  []any{apiGroup().Label, apiID, podGroup().Label, podID},
				},
				table(`CREATE TABLE grouped (
					slack_id TEXT PRIMARY KEY, repo TEXT, slack_team TEXT, number INTEGER
				) WITHOUT ROWID`),
				{
					query: `INSERT INTO grouped (slack_id, repo, slack_team, number) VALUES (?, ?, ?, 1), (?, ?, ?, ?)`,
					args:  []any{apiID, repo, workspaceA, podID, repo, workspaceA, smallestInteger},
				},
				table(`CREATE VIEW repo_group AS
					SELECT repo, slack_team, slack_id, '' AS added_at FROM grouped WHERE abs(number) > 0`),
			},
			call: repoGroups, step: readingGroups,
		},
		"the groups a choice is made among": {
			before: made,
			tamper: []statement{
				gone("repo_group"),
				table(`CREATE TABLE grouped (
					repo TEXT, slack_team TEXT, slack_id TEXT PRIMARY KEY, number INTEGER
				) WITHOUT ROWID`),
				{
					query: `INSERT INTO grouped (repo, slack_team, slack_id, number) VALUES (?, ?, ?, 1), (?, ?, ?, ?)`,
					args:  []any{repo, workspaceA, apiID, repo, workspaceA, podID, smallestInteger},
				},
				table(`CREATE VIEW repo_group AS SELECT repo, slack_team,
					CASE WHEN abs(number) > 0 THEN slack_id END AS slack_id, '' AS added_at FROM grouped`),
			},
			call: listNoGroup, step: "reading the Slack IDs",
		},
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
			readBack, err := call(t.Context(), store.New(dir, false).ReadOnly())

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

func TestAKeptFileWithATableInTheWayIsReportedAndLeftAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	// A file at no version is one the store makes its schema in; a table
	// another program left in it under one of the store's names stops that,
	// and the store says so rather than write around it.
	dir := t.TempDir()
	execKept(t, dir, `CREATE TABLE favorite_dir (other TEXT)`)

	// Act
	err := store.New(dir, false).Favor(t.Context(), apiDir, theTime())

	// Assert
	if err == nil || !strings.Contains(err.Error(), "preparing the kept data") {
		t.Errorf("Favor = %v, want the schema step to report the table in its way", err)
	}

	var tables int

	err = openKeptFile(t, dir).QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'`).Scan(&tables)
	if err != nil || tables != 1 || keptPragma(t, dir, "user_version") != 0 {
		t.Errorf("the file holds %d tables (err %v) at version %d, want only the one it had, at none",
			tables, err, keptPragma(t, dir, "user_version"))
	}
}
