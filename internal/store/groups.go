// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"
)

// groupsSchema makes the repository group tables. A repository's groups are
// the Slack user groups it may tag in a workspace, by ID, their labels living
// in slack_entity; its choice records that groups were last chosen, even none,
// and one child row per group chosen. A group dropped from the list drops out
// of the choice.
func groupsSchema() []string {
	return []string{
		`CREATE TABLE repo_group (
			repo       TEXT NOT NULL,
			slack_team TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			added_at   TEXT NOT NULL,
			PRIMARY KEY (repo, slack_team, slack_id),
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT
		) STRICT`,
		`CREATE INDEX repo_group_by_entity ON repo_group (slack_id)`,
		`CREATE TABLE repo_choice (
			repo      TEXT NOT NULL PRIMARY KEY,
			chosen_at TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE repo_choice_group (
			repo       TEXT NOT NULL,
			slack_team TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			PRIMARY KEY (repo, slack_team, slack_id),
			FOREIGN KEY (repo) REFERENCES repo_choice(repo) ON DELETE CASCADE,
			FOREIGN KEY (repo, slack_team, slack_id)
				REFERENCES repo_group(repo, slack_team, slack_id) ON DELETE CASCADE
		) STRICT`,
	}
}

// repoIn is a repository as a Slack workspace sees it.
type repoIn struct {
	repo      string
	workspace string
}

// RepoGroups is the Slack user groups of a workspace a repository may tag, by
// label. A row of the wrong shape is left out, and each label is sanitized and
// capped. A disabled store, or a repository with none, reports none.
func (s Store) RepoGroups(ctx context.Context, repo, workspace string) ([]SlackTarget, error) {
	if repo == "" {
		return nil, nil
	}

	if workspace == "" {
		return nil, ErrNoWorkspace
	}

	database, found, err := s.readKept(ctx)
	if err != nil || !found {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx,
		`SELECT entity.slack_id, entity.label FROM repo_group AS listed
			JOIN slack_entity AS entity ON entity.slack_id = listed.slack_id
			WHERE listed.repo = ? AND listed.slack_team = ?
			ORDER BY entity.label, entity.slack_id`, repo, workspace)
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

// SetRepoGroups replaces the Slack user groups of a workspace a repository
// may tag, in one transaction, leaving other workspaces' groups as they are.
// It refuses any ID not a group's with ErrInvalidSlackID, and groups under no
// workspace with ErrNoWorkspace. A group dropped from the list drops out of
// the last choice too. A disabled or read-only store records nothing.
func (s Store) SetRepoGroups(ctx context.Context, repo, workspace string, groups []SlackTarget, now time.Time) error {
	if repo == "" {
		return nil
	}

	if workspace == "" {
		return ErrNoWorkspace
	}

	for _, group := range groups {
		if !isSlackID(group.ID, groupIDPrefixes) {
			return ErrInvalidSlackID
		}
	}

	where := repoIn{repo: repo, workspace: workspace}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		err := dropUnlistedGroups(ctx, transaction, where, groups)
		if err != nil {
			return err
		}

		err = listGroups(ctx, transaction, where, groups, now)
		if err != nil {
			return err
		}

		return pruneSlackEntities(ctx, transaction)
	})
}

// dropUnlistedGroups deletes the repository's groups in the workspace that
// groups no longer holds, one by one, so a group kept keeps its place in the
// last choice.
func dropUnlistedGroups(ctx context.Context, transaction *sql.Tx, where repoIn, groups []SlackTarget) error {
	listed, err := listedGroupIDs(ctx, transaction, where)
	if err != nil {
		return err
	}

	for _, slackID := range listed {
		if slices.ContainsFunc(groups, func(group SlackTarget) bool { return group.ID == slackID }) {
			continue
		}

		_, err = transaction.ExecContext(ctx,
			`DELETE FROM repo_group WHERE repo = ? AND slack_team = ? AND slack_id = ?`,
			where.repo, where.workspace, slackID)
		if err != nil {
			return fmt.Errorf("dropping a repository group: %w", err)
		}
	}

	return nil
}

// listedGroupIDs is every group ID listed for the repository in the
// workspace, as stored.
func listedGroupIDs(ctx context.Context, transaction *sql.Tx, where repoIn) ([]string, error) {
	rows, err := transaction.QueryContext(ctx,
		`SELECT slack_id FROM repo_group WHERE repo = ? AND slack_team = ?`, where.repo, where.workspace)
	if err != nil {
		return nil, fmt.Errorf("reading the repository's groups: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanIDs(rows)
}

// listGroups keeps each group's label and lists it for the repository, leaving
// one already listed as it was.
func listGroups(ctx context.Context, transaction *sql.Tx, where repoIn, groups []SlackTarget, now time.Time) error {
	for _, group := range groups {
		err := keepSlackEntity(ctx, transaction, group, now)
		if err != nil {
			return err
		}

		_, err = transaction.ExecContext(ctx,
			`INSERT INTO repo_group (repo, slack_team, slack_id, added_at) VALUES (?, ?, ?, ?)
				ON CONFLICT(repo, slack_team, slack_id) DO NOTHING`,
			where.repo, where.workspace, group.ID, timestamp(now))
		if err != nil {
			return fmt.Errorf("listing a repository group: %w", err)
		}
	}

	return nil
}

// LastGroups is the group IDs of a workspace last chosen for a repository's
// announcement, by ID, and whether a choice was recorded at all, in any
// workspace — a choice of no group is still one. An ID of the wrong shape is
// left out. A disabled store reports no choice.
func (s Store) LastGroups(ctx context.Context, repo, workspace string) ([]string, bool, error) {
	if repo == "" {
		return nil, false, nil
	}

	if workspace == "" {
		return nil, false, ErrNoWorkspace
	}

	database, found, err := s.readKept(ctx)
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

	ids, err := chosenGroupIDs(ctx, database, repoIn{repo: repo, workspace: workspace})
	if err != nil {
		return nil, false, err
	}

	return ids, true, nil
}

// chosenGroupIDs reads the group IDs of a repository's last choice in the
// workspace, by ID, leaving out any of the wrong shape.
func chosenGroupIDs(ctx context.Context, database *sql.DB, where repoIn) ([]string, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT slack_id FROM repo_choice_group WHERE repo = ? AND slack_team = ? ORDER BY slack_id`,
		where.repo, where.workspace)
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

// RecordGroups remembers the groups of a workspace just chosen for a
// repository's announcement, replacing that workspace's last choice in one
// transaction. Only the repository's own groups are remembered: an
// announcement also tags groups linked to owning teams, which need not be
// listed, and refusing the whole choice over one of those would lose the rest
// of it. A disabled or read-only store records nothing.
func (s Store) RecordGroups(ctx context.Context, repo, workspace string, ids []string, now time.Time) error {
	if repo == "" {
		return nil
	}

	if workspace == "" {
		return ErrNoWorkspace
	}

	where := repoIn{repo: repo, workspace: workspace}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx,
			`INSERT INTO repo_choice (repo, chosen_at) VALUES (?, ?)
				ON CONFLICT(repo) DO UPDATE SET chosen_at = excluded.chosen_at`,
			repo, timestamp(now))
		if err != nil {
			return fmt.Errorf("recording the choice of groups: %w", err)
		}

		_, err = transaction.ExecContext(ctx,
			`DELETE FROM repo_choice_group WHERE repo = ? AND slack_team = ?`, repo, workspace)
		if err != nil {
			return fmt.Errorf("clearing the last choice of groups: %w", err)
		}

		return chooseGroups(ctx, transaction, where, ids)
	})
}

// chooseGroups records each chosen group the repository lists, leaving out any
// other.
func chooseGroups(ctx context.Context, transaction *sql.Tx, where repoIn, ids []string) error {
	listed, err := listedGroupIDs(ctx, transaction, where)
	if err != nil {
		return err
	}

	for _, slackID := range ids {
		if !slices.Contains(listed, slackID) {
			continue
		}

		_, err = transaction.ExecContext(ctx,
			`INSERT INTO repo_choice_group (repo, slack_team, slack_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`,
			where.repo, where.workspace, slackID)
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
