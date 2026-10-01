// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ownersKeptIn is the migration that makes the owner tables: a read needs the
// file migrated at least this far.
const ownersKeptIn = 1

// SlackTarget is a Slack user or user group, by its ID and the label it was
// last seen with — the store's own shape, in plain strings.
type SlackTarget struct {
	ID    string
	Label string
}

// OwnerLink is what was decided for one forge owner: whether they are on Slack,
// and if so, as whom.
type OwnerLink struct {
	Owner   string
	OnSlack bool
	Slack   SlackTarget
}

// ownersMigration makes the owner tables. A decision row records that an owner
// was asked about; its one owner_slack child says whom they are on Slack, and no
// child says they are not on Slack, so neither is asked again. A Slack user or
// group's label lives once in slack_entity, whatever links to it.
func ownersMigration() []string {
	return []string{
		`CREATE TABLE slack_entity (
			slack_id TEXT NOT NULL PRIMARY KEY,
			label    TEXT NOT NULL,
			seen_at  TEXT NOT NULL
		) STRICT`,
		`CREATE TABLE owner_decision (
			forge_host TEXT NOT NULL,
			owner      TEXT NOT NULL,
			decided_at TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner)
		) STRICT`,
		`CREATE TABLE owner_slack (
			forge_host TEXT NOT NULL,
			owner      TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner),
			FOREIGN KEY (forge_host, owner) REFERENCES owner_decision(forge_host, owner) ON DELETE CASCADE,
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT
		) STRICT`,
		`CREATE INDEX owner_slack_by_entity ON owner_slack (slack_id)`,
	}
}

// OwnerLinks is every owner decided on a forge host, by owner. A disabled
// store, or a host with nothing decided, reports none.
func (s Store) OwnerLinks(ctx context.Context, forgeHost string) ([]OwnerLink, error) {
	if forgeHost == "" {
		return nil, nil
	}

	database, found, err := s.readKept(ctx, ownersKeptIn)
	if err != nil || !found {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx,
		`SELECT decision.owner, entity.slack_id, entity.label
			FROM owner_decision AS decision
			LEFT JOIN owner_slack AS link USING (forge_host, owner)
			LEFT JOIN slack_entity AS entity ON entity.slack_id = link.slack_id
			WHERE decision.forge_host = ? ORDER BY decision.owner`, forgeHost)
	if err != nil {
		return nil, fmt.Errorf("reading the owner links: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var links []OwnerLink

	for rows.Next() {
		var (
			owner          string
			slackID, label sql.NullString
		)

		err = rows.Scan(&owner, &slackID, &label)
		if err != nil {
			return nil, fmt.Errorf("reading an owner link: %w", err)
		}

		links = append(links, OwnerLink{
			Owner: owner, OnSlack: slackID.Valid, Slack: SlackTarget{ID: slackID.String, Label: label.String},
		})
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the owner links: %w", err)
	}

	return links, nil
}

// LinkOwner records what was decided for a forge owner on a host: target is
// whom they are on Slack, and nil that they are not on Slack. It replaces any
// earlier decision. A disabled or read-only store records nothing.
func (s Store) LinkOwner(ctx context.Context, forgeHost, owner string, target *SlackTarget, now time.Time) error {
	if forgeHost == "" {
		return nil
	}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		return writeOwnerLink(ctx, transaction, forgeHost, owner, target, now)
	})
}

// writeOwnerLink upserts the decision and replaces its Slack link.
func writeOwnerLink(
	ctx context.Context, transaction *sql.Tx, forgeHost, owner string, target *SlackTarget, now time.Time,
) error {
	_, err := transaction.ExecContext(ctx,
		`INSERT INTO owner_decision (forge_host, owner, decided_at) VALUES (?, ?, ?)
			ON CONFLICT(forge_host, owner) DO UPDATE SET decided_at = excluded.decided_at`,
		forgeHost, owner, timestamp(now))
	if err != nil {
		return fmt.Errorf("recording the owner decision: %w", err)
	}

	_, err = transaction.ExecContext(ctx,
		`DELETE FROM owner_slack WHERE forge_host = ? AND owner = ?`, forgeHost, owner)
	if err != nil {
		return fmt.Errorf("clearing the owner's Slack link: %w", err)
	}

	if target == nil {
		return nil
	}

	err = keepSlackEntity(ctx, transaction, *target, now)
	if err != nil {
		return err
	}

	_, err = transaction.ExecContext(ctx,
		`INSERT INTO owner_slack (forge_host, owner, slack_id) VALUES (?, ?, ?)`, forgeHost, owner, target.ID)
	if err != nil {
		return fmt.Errorf("linking the owner to Slack: %w", err)
	}

	return nil
}

// keepSlackEntity upserts a Slack user or group with the label it was just seen
// with.
func keepSlackEntity(ctx context.Context, transaction *sql.Tx, target SlackTarget, now time.Time) error {
	_, err := transaction.ExecContext(ctx,
		`INSERT INTO slack_entity (slack_id, label, seen_at) VALUES (?, ?, ?)
			ON CONFLICT(slack_id) DO UPDATE SET label = excluded.label, seen_at = excluded.seen_at`,
		target.ID, target.Label, timestamp(now))
	if err != nil {
		return fmt.Errorf("keeping the Slack entity: %w", err)
	}

	return nil
}
