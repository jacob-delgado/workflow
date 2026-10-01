// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"
)

// groupsKeptIn is the migration that makes the repository group tables: a read
// needs the file migrated at least this far.
const groupsKeptIn = 2

// ErrGroupNotListed reports a chosen group that is not one of the repository's
// groups.
var ErrGroupNotListed = errors.New("the group is not one of the repository's groups")

// groupsMigration makes the repository group tables. A repository's groups are
// the Slack user groups it may tag, by ID, their labels living in slack_entity;
// its choice records that groups were last chosen, even none, and one child row
// per group chosen. A group dropped from the list drops out of the choice.
func groupsMigration() []string {
	return []string{
		`CREATE TABLE repo_group (
			repo     TEXT NOT NULL,
			slack_id TEXT NOT NULL,
			added_at TEXT NOT NULL,
			PRIMARY KEY (repo, slack_id),
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT
		) STRICT`,
		`CREATE INDEX repo_group_by_entity ON repo_group (slack_id)`,
		`CREATE TABLE repo_choice (
			repo      TEXT NOT NULL PRIMARY KEY,
			chosen_at TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE repo_choice_group (
			repo     TEXT NOT NULL,
			slack_id TEXT NOT NULL,
			PRIMARY KEY (repo, slack_id),
			FOREIGN KEY (repo) REFERENCES repo_choice(repo) ON DELETE CASCADE,
			FOREIGN KEY (repo, slack_id) REFERENCES repo_group(repo, slack_id) ON DELETE CASCADE
		) STRICT`,
	}
}

// RepoGroups is the Slack user groups a repository may tag, by label. A row of
// the wrong shape is left out, and each label is sanitized and capped. A
// disabled store, or a repository with none, reports none.
func (s Store) RepoGroups(ctx context.Context, repo string) ([]SlackTarget, error) {
	if repo == "" {
		return nil, nil
	}

	database, found, err := s.readKept(ctx, groupsKeptIn)
	if err != nil || !found {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx,
		`SELECT entity.slack_id, entity.label FROM repo_group AS listed
			JOIN slack_entity AS entity ON entity.slack_id = listed.slack_id
			WHERE listed.repo = ? ORDER BY entity.label, entity.slack_id`, repo)
	if err != nil {
		return nil, fmt.Errorf("reading the repository's groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var groups []SlackTarget

	for rows.Next() {
		var group SlackTarget

		err = rows.Scan(&group.ID, &group.Label)
		if err != nil {
			return nil, fmt.Errorf("reading a repository group: %w", err)
		}

		if isSlackID(group.ID, groupIDPrefixes) {
			groups = append(groups, SlackTarget{ID: group.ID, Label: cleanLabel(group.Label)})
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the repository's groups: %w", err)
	}

	return groups, nil
}

// SetRepoGroups replaces the Slack user groups a repository may tag, in one
// transaction, refusing any ID not a group's with ErrInvalidSlackID. A group
// dropped from the list drops out of the last choice too. A disabled or
// read-only store records nothing.
func (s Store) SetRepoGroups(ctx context.Context, repo string, groups []SlackTarget, now time.Time) error {
	if repo == "" {
		return nil
	}

	for _, group := range groups {
		if !isSlackID(group.ID, groupIDPrefixes) {
			return ErrInvalidSlackID
		}
	}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		err := dropUnlistedGroups(ctx, transaction, repo, groups)
		if err != nil {
			return err
		}

		err = listGroups(ctx, transaction, repo, groups, now)
		if err != nil {
			return err
		}

		return pruneSlackEntities(ctx, transaction)
	})
}

// dropUnlistedGroups deletes the repository's groups that groups no longer
// holds, one by one, so a group kept keeps its place in the last choice.
func dropUnlistedGroups(ctx context.Context, transaction *sql.Tx, repo string, groups []SlackTarget) error {
	listed, err := listedGroupIDs(ctx, transaction, repo)
	if err != nil {
		return err
	}

	for _, slackID := range listed {
		if slices.ContainsFunc(groups, func(group SlackTarget) bool { return group.ID == slackID }) {
			continue
		}

		_, err = transaction.ExecContext(ctx,
			`DELETE FROM repo_group WHERE repo = ? AND slack_id = ?`, repo, slackID)
		if err != nil {
			return fmt.Errorf("dropping a repository group: %w", err)
		}
	}

	return nil
}

// listedGroupIDs is every group ID listed for the repository, as stored.
func listedGroupIDs(ctx context.Context, transaction *sql.Tx, repo string) ([]string, error) {
	rows, err := transaction.QueryContext(ctx, `SELECT slack_id FROM repo_group WHERE repo = ?`, repo)
	if err != nil {
		return nil, fmt.Errorf("reading the repository's groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanIDs(rows)
}

// listGroups keeps each group's label and lists it for the repository, leaving
// one already listed as it was.
func listGroups(ctx context.Context, transaction *sql.Tx, repo string, groups []SlackTarget, now time.Time) error {
	for _, group := range groups {
		err := keepSlackEntity(ctx, transaction, group, now)
		if err != nil {
			return err
		}

		_, err = transaction.ExecContext(ctx,
			`INSERT INTO repo_group (repo, slack_id, added_at) VALUES (?, ?, ?)
				ON CONFLICT(repo, slack_id) DO NOTHING`,
			repo, group.ID, timestamp(now))
		if err != nil {
			return fmt.Errorf("listing a repository group: %w", err)
		}
	}

	return nil
}

// LastGroups is the group IDs last chosen for a repository's announcement, by
// ID, and whether a choice was recorded at all — a choice of no group is still
// one. An ID of the wrong shape is left out. A disabled store reports no
// choice.
func (s Store) LastGroups(ctx context.Context, repo string) ([]string, bool, error) {
	if repo == "" {
		return nil, false, nil
	}

	database, found, err := s.readKept(ctx, groupsKeptIn)
	if err != nil || !found {
		return nil, false, err
	}
	defer func() { _ = database.Close() }()

	var chosen bool

	err = database.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM repo_choice WHERE repo = ?)`, repo).Scan(&chosen)
	if err != nil || !chosen {
		return nil, false, wrapIfFailed("reading the last choice of groups", err)
	}

	ids, err := chosenGroupIDs(ctx, database, repo)
	if err != nil {
		return nil, false, err
	}

	return ids, true, nil
}

// chosenGroupIDs reads the group IDs of a repository's last choice, by ID,
// leaving out any of the wrong shape.
func chosenGroupIDs(ctx context.Context, database *sql.DB, repo string) ([]string, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT slack_id FROM repo_choice_group WHERE repo = ? ORDER BY slack_id`, repo)
	if err != nil {
		return nil, fmt.Errorf("reading the last choice of groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	stored, err := scanIDs(rows)
	if err != nil {
		return nil, err
	}

	ids := []string{}

	for _, slackID := range stored {
		if isSlackID(slackID, groupIDPrefixes) {
			ids = append(ids, slackID)
		}
	}

	return ids, nil
}

// RecordGroups remembers the groups just chosen for a repository's
// announcement, replacing the last choice in one transaction. Every ID must be
// one of the repository's groups, or the choice is refused with
// ErrGroupNotListed. A disabled or read-only store records nothing.
func (s Store) RecordGroups(ctx context.Context, repo string, ids []string, now time.Time) error {
	if repo == "" {
		return nil
	}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx,
			`INSERT INTO repo_choice (repo, chosen_at) VALUES (?, ?)
				ON CONFLICT(repo) DO UPDATE SET chosen_at = excluded.chosen_at`,
			repo, timestamp(now))
		if err != nil {
			return fmt.Errorf("recording the choice of groups: %w", err)
		}

		_, err = transaction.ExecContext(ctx, `DELETE FROM repo_choice_group WHERE repo = ?`, repo)
		if err != nil {
			return fmt.Errorf("clearing the last choice of groups: %w", err)
		}

		return chooseGroups(ctx, transaction, repo, ids)
	})
}

// chooseGroups records each chosen group, refusing one the repository does not
// list.
func chooseGroups(ctx context.Context, transaction *sql.Tx, repo string, ids []string) error {
	listed, err := listedGroupIDs(ctx, transaction, repo)
	if err != nil {
		return err
	}

	for _, slackID := range ids {
		if !slices.Contains(listed, slackID) {
			return ErrGroupNotListed
		}

		_, err = transaction.ExecContext(ctx,
			`INSERT INTO repo_choice_group (repo, slack_id) VALUES (?, ?) ON CONFLICT DO NOTHING`, repo, slackID)
		if err != nil {
			return fmt.Errorf("recording a chosen group: %w", err)
		}
	}

	return nil
}

// scanIDs reads one ID per row.
func scanIDs(rows *sql.Rows) ([]string, error) {
	var ids []string

	for rows.Next() {
		var slackID string

		err := rows.Scan(&slackID)
		if err != nil {
			return nil, fmt.Errorf("reading a Slack ID: %w", err)
		}

		ids = append(ids, slackID)
	}

	err := rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the Slack IDs: %w", err)
	}

	return ids, nil
}

// wrapIfFailed wraps a failed read with what was being read, and passes no
// error through as none.
func wrapIfFailed(reading string, err error) error {
	if err == nil {
		return nil
	}

	return fmt.Errorf("%s: %w", reading, err)
}
