// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// ownersKeptIn is the migration that makes the owner tables: a read needs the
// file migrated at least this far.
const ownersKeptIn = 1

// The Slack ID prefixes a forge owner may link to: a user (U, or W in an
// Enterprise Grid) for a user owner, and a user group (S) for a team owner.
const (
	userIDPrefixes  = "UW"
	groupIDPrefixes = "S"
)

// labelRunes caps a label read back, so a tampered file cannot widen a row past
// any surface's layout.
const labelRunes = 80

// ErrInvalidOwner reports a forge owner not shaped like a CODEOWNERS user or
// team.
var ErrInvalidOwner = errors.New("not a forge owner")

// ErrInvalidSlackID reports a Slack ID of the wrong shape, or of the wrong kind
// for what it is linked to.
var ErrInvalidSlackID = errors.New("not a Slack ID of the right kind")

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

// OwnerLinks is every owner decided on a forge host, by owner in lower case. A
// row of the wrong shape — a tampered or corrupt file's — is left out, so that
// owner reads as never decided, and each label is sanitized and capped. A
// disabled store, or a host with nothing decided, reports none.
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
		`SELECT decision.owner, link.slack_id, entity.label
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

		link, valid := ownerLinkFrom(owner, slackID, label)
		if valid {
			links = append(links, link)
		}
	}

	err = rows.Err()
	if err != nil {
		return nil, fmt.Errorf("reading the owner links: %w", err)
	}

	return links, nil
}

// ownerLinkFrom validates one row read back: an owner of the wrong shape, or a
// link to a Slack ID of the wrong shape or kind for it, or to no Slack entity,
// is no link at all.
func ownerLinkFrom(owner string, slackID, label sql.NullString) (OwnerLink, bool) {
	linkedWell := isSlackIDFor(owner, slackID.String) && label.Valid
	if !isOwner(owner) || slackID.Valid && !linkedWell {
		return OwnerLink{}, false
	}

	return OwnerLink{
		Owner:   strings.ToLower(owner),
		OnSlack: slackID.Valid,
		Slack:   SlackTarget{ID: slackID.String, Label: cleanLabel(label.String)},
	}, true
}

// LinkOwner records what was decided for a forge owner on a host: target is
// whom they are on Slack, and nil that they are not on Slack. It replaces any
// earlier decision, and refuses an owner or Slack ID of the wrong shape with
// ErrInvalidOwner or ErrInvalidSlackID. The owner is kept in lower case, since
// both forges read user and team names without regard to case. A disabled or
// read-only store records nothing.
func (s Store) LinkOwner(ctx context.Context, forgeHost, owner string, target *SlackTarget, now time.Time) error {
	switch {
	case forgeHost == "":
		return nil
	case !isOwner(owner):
		return ErrInvalidOwner
	case target != nil && !isSlackIDFor(owner, target.ID):
		return ErrInvalidSlackID
	}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		err := writeOwnerLink(ctx, transaction, forgeHost, strings.ToLower(owner), target, now)
		if err != nil {
			return err
		}

		return pruneSlackEntities(ctx, transaction)
	})
}

// ForgetOwner drops what was decided for a forge owner on a host, so they are
// asked again, whatever the case of owner. A disabled or read-only store
// forgets nothing.
func (s Store) ForgetOwner(ctx context.Context, forgeHost, owner string) error {
	if forgeHost == "" {
		return nil
	}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		_, err := transaction.ExecContext(ctx,
			`DELETE FROM owner_decision WHERE forge_host = ? AND owner = ?`, forgeHost, strings.ToLower(owner))
		if err != nil {
			return fmt.Errorf("forgetting the owner: %w", err)
		}

		return pruneSlackEntities(ctx, transaction)
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
// with, sanitized on the way in as well as on the way out.
func keepSlackEntity(ctx context.Context, transaction *sql.Tx, target SlackTarget, now time.Time) error {
	_, err := transaction.ExecContext(ctx,
		`INSERT INTO slack_entity (slack_id, label, seen_at) VALUES (?, ?, ?)
			ON CONFLICT(slack_id) DO UPDATE SET label = excluded.label, seen_at = excluded.seen_at`,
		target.ID, cleanLabel(target.Label), timestamp(now))
	if err != nil {
		return fmt.Errorf("keeping the Slack entity: %w", err)
	}

	return nil
}

// isOwner reports a CODEOWNERS user or team: /-separated segments of ASCII
// letters, digits, dots, underscores and hyphens, the first starting with a
// letter or digit.
func isOwner(owner string) bool {
	if owner == "" || !isAlphanumeric(rune(owner[0])) {
		return false
	}

	for segment := range strings.SplitSeq(owner, "/") {
		if segment == "" || strings.ContainsFunc(segment, isNotOwnerCharacter) {
			return false
		}
	}

	return true
}

// isNotOwnerCharacter reports a character no owner's name holds.
func isNotOwnerCharacter(character rune) bool {
	return !isAlphanumeric(character) && !strings.ContainsRune("._-", character)
}

// isAlphanumeric reports an ASCII letter or digit.
func isAlphanumeric(character rune) bool {
	return 'a' <= character && character <= 'z' || isUpperOrDigit(character)
}

// isUpperOrDigit reports an ASCII capital or digit, all a Slack ID holds.
func isUpperOrDigit(character rune) bool {
	return 'A' <= character && character <= 'Z' || '0' <= character && character <= '9'
}

// isSlackIDFor reports a Slack ID of the kind owner links to: a user's for a
// user owner, and a user group's for a team owner, whose name holds a slash.
func isSlackIDFor(owner, slackID string) bool {
	prefixes := userIDPrefixes
	if strings.Contains(owner, "/") {
		prefixes = groupIDPrefixes
	}

	return isSlackID(slackID, prefixes)
}

// isSlackID reports an ID that starts with one of prefixes and goes on with at
// least two capitals or digits, as Slack's IDs do.
func isSlackID(slackID, prefixes string) bool {
	const shortest = 3

	return len(slackID) >= shortest &&
		strings.ContainsRune(prefixes, rune(slackID[0])) &&
		!strings.ContainsFunc(slackID[1:], func(character rune) bool { return !isUpperOrDigit(character) })
}

// cleanLabel neutralizes any terminal control in a label and caps its length.
func cleanLabel(label string) string {
	clean := sanitize.Line(label)
	if utf8.RuneCountInString(clean) <= labelRunes {
		return clean
	}

	return string([]rune(clean)[:labelRunes])
}
