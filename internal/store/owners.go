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

// linkWorkspacesKeptIn is the migration that keeps each Slack link, listed
// group and chosen group with its workspace: every read of them filters by
// one, so it needs the file migrated at least this far.
const linkWorkspacesKeptIn = 5

// indexOwnerSlackByEntity indexes the owner links by the Slack ID they link
// to, made again by each migration that remakes owner_slack.
const indexOwnerSlackByEntity = `CREATE INDEX owner_slack_by_entity ON owner_slack (slack_id)`

// The kinds of forge owner a decision keeps.
const (
	kindUser = "user"
	kindTeam = "team"
)

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

// ErrNoWorkspace reports a Slack link read or made under no Slack workspace,
// which would reach the links kept before workspaces were, whose workspace no
// one knows.
var ErrNoWorkspace = errors.New("no Slack workspace to keep Slack links under")

// ErrInvalidSlackID reports a Slack ID of the wrong shape, or of the wrong kind
// for what it is linked to.
var ErrInvalidSlackID = errors.New("not a Slack ID of the right kind")

// SlackTarget is a Slack user or user group, by its ID and the label it was
// last seen with — the store's own shape, in plain strings.
type SlackTarget struct {
	ID    string
	Label string
}

// OwnerLink is what was decided for one forge owner: whether they are a team,
// which links to a Slack user group, or a person, who links to a Slack user;
// whether they are on Slack; and if so, as whom.
type OwnerLink struct {
	Owner   string
	Team    bool
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
		indexOwnerSlackByEntity,
	}
}

// workspacesMigration keys Slack links by the Slack workspace they were made
// in. A Slack user or group belongs to one workspace, so the workspace hangs
// off slack_entity and every link reaches it through the ID, which keeps the
// tables in 3NF; an entity kept before has no workspace (”), so nothing
// linked to it is read until it is linked again. An owner may now have one
// link per workspace, so owner_slack is remade keyed by the ID too, its rows
// copied as they are.
// linkWorkspacesMigration later moves the workspace to the rows that link,
// since one ID can be seen from more than one workspace.
func workspacesMigration() []string {
	return []string{
		`ALTER TABLE slack_entity ADD COLUMN slack_team TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE owner_slack_per_workspace (
			forge_host TEXT NOT NULL,
			owner      TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner, slack_id),
			FOREIGN KEY (forge_host, owner) REFERENCES owner_decision(forge_host, owner) ON DELETE CASCADE,
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT
		) STRICT`,
		`INSERT INTO owner_slack_per_workspace (forge_host, owner, slack_id)
			SELECT forge_host, owner, slack_id FROM owner_slack`,
		`DROP TABLE owner_slack`,
		`ALTER TABLE owner_slack_per_workspace RENAME TO owner_slack`,
		indexOwnerSlackByEntity,
	}
}

// ownerKindsMigration keeps whether each owner is a person or a team with the
// decision, which the caller says: CODEOWNERS spells a top-level GitLab group
// @acme, as it spells a user, so the name cannot. Every decision kept before
// was made by the name, a team holding a slash, so that is the kind each is
// given.
func ownerKindsMigration() []string {
	return []string{
		`ALTER TABLE owner_decision ADD COLUMN owner_kind TEXT NOT NULL DEFAULT 'user'
			CHECK (owner_kind IN ('user', 'team'))`,
		`UPDATE owner_decision SET owner_kind = 'team' WHERE instr(owner, '/') > 0`,
	}
}

// linkWorkspacesMigration moves the workspace from the Slack entity to each
// row that links to one. An ID can be seen from more than one workspace — an
// Enterprise Grid's W user, a Slack Connect member, an org-wide S group — so
// the workspace a link, a listed group or a chosen group was made in is the
// row's, not the ID's, whose label alone depends on it. Each row is copied
// with its entity's workspace, which is where it was read until now. That an
// owner is not on Slack is kept per workspace too, in owner_not_on_slack: a
// decision with neither a link nor such a row, as one kept before is, is not
// on Slack in any workspace.
func linkWorkspacesMigration() []string {
	return []string{
		`CREATE TABLE owner_slack_in_workspace (
			forge_host TEXT NOT NULL,
			owner      TEXT NOT NULL,
			slack_team TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner, slack_team),
			FOREIGN KEY (forge_host, owner) REFERENCES owner_decision(forge_host, owner) ON DELETE CASCADE,
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT
		) STRICT`,
		`INSERT INTO owner_slack_in_workspace (forge_host, owner, slack_team, slack_id)
			SELECT link.forge_host, link.owner, entity.slack_team, link.slack_id
			FROM owner_slack AS link JOIN slack_entity AS entity USING (slack_id) WHERE true
			ON CONFLICT DO NOTHING`,
		`CREATE TABLE owner_not_on_slack (
			forge_host TEXT NOT NULL,
			owner      TEXT NOT NULL,
			slack_team TEXT NOT NULL,
			PRIMARY KEY (forge_host, owner, slack_team),
			FOREIGN KEY (forge_host, owner) REFERENCES owner_decision(forge_host, owner) ON DELETE CASCADE
		) STRICT`,
		`CREATE TABLE repo_group_in_workspace (
			repo       TEXT NOT NULL,
			slack_team TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			added_at   TEXT NOT NULL,
			PRIMARY KEY (repo, slack_team, slack_id),
			FOREIGN KEY (slack_id) REFERENCES slack_entity(slack_id) ON DELETE RESTRICT
		) STRICT`,
		`INSERT INTO repo_group_in_workspace (repo, slack_team, slack_id, added_at)
			SELECT listed.repo, entity.slack_team, listed.slack_id, listed.added_at
			FROM repo_group AS listed JOIN slack_entity AS entity USING (slack_id)`,
		`CREATE TABLE repo_choice_group_in_workspace (
			repo       TEXT NOT NULL,
			slack_team TEXT NOT NULL,
			slack_id   TEXT NOT NULL,
			PRIMARY KEY (repo, slack_team, slack_id),
			FOREIGN KEY (repo) REFERENCES repo_choice(repo) ON DELETE CASCADE,
			FOREIGN KEY (repo, slack_team, slack_id)
				REFERENCES repo_group_in_workspace(repo, slack_team, slack_id) ON DELETE CASCADE
		) STRICT`,
		`INSERT INTO repo_choice_group_in_workspace (repo, slack_team, slack_id)
			SELECT chosen.repo, entity.slack_team, chosen.slack_id
			FROM repo_choice_group AS chosen JOIN slack_entity AS entity USING (slack_id)`,
		`DROP TABLE repo_choice_group`,
		`DROP TABLE repo_group`,
		`DROP TABLE owner_slack`,
		`ALTER TABLE owner_slack_in_workspace RENAME TO owner_slack`,
		`ALTER TABLE repo_group_in_workspace RENAME TO repo_group`,
		`ALTER TABLE repo_choice_group_in_workspace RENAME TO repo_choice_group`,
		indexOwnerSlackByEntity,
		`CREATE INDEX repo_group_by_entity ON repo_group (slack_id)`,
		`ALTER TABLE slack_entity DROP COLUMN slack_team`,
	}
}

// OwnerLinks is every owner decided on a forge host as a Slack workspace sees
// them, by owner in lower case: linked there, decided not on Slack there, or
// decided not on Slack before workspaces were and in none since. An owner
// decided only in other workspaces is left out, so they read as never decided
// here and are asked again. A row of the wrong shape — a tampered or corrupt
// file's — is left out too, and each label is sanitized and capped. A
// disabled store, or a host with nothing decided, reports none.
func (s Store) OwnerLinks(ctx context.Context, forgeHost, workspace string) ([]OwnerLink, error) {
	if forgeHost == "" {
		return nil, nil
	}

	if workspace == "" {
		return nil, ErrNoWorkspace
	}

	database, found, err := s.readKept(ctx, linkWorkspacesKeptIn)
	if err != nil || !found {
		return nil, err
	}
	defer func() { _ = database.Close() }()

	rows, err := database.QueryContext(ctx,
		`SELECT decision.owner, decision.owner_kind, here.slack_id, entity.label
			FROM owner_decision AS decision
			LEFT JOIN owner_slack AS here ON here.forge_host = decision.forge_host
				AND here.owner = decision.owner AND here.slack_team = ?
			LEFT JOIN slack_entity AS entity ON entity.slack_id = here.slack_id
			WHERE decision.forge_host = ? AND (here.slack_id IS NOT NULL
				OR EXISTS (SELECT 1 FROM owner_not_on_slack AS absent
					WHERE absent.forge_host = decision.forge_host AND absent.owner = decision.owner
					AND absent.slack_team = ?)
				OR NOT EXISTS (SELECT 1 FROM owner_slack AS anywhere
					WHERE anywhere.forge_host = decision.forge_host AND anywhere.owner = decision.owner)
				AND NOT EXISTS (SELECT 1 FROM owner_not_on_slack AS anywhere
					WHERE anywhere.forge_host = decision.forge_host AND anywhere.owner = decision.owner))
			ORDER BY decision.owner`, workspace, forgeHost, workspace)
	if err != nil {
		return nil, fmt.Errorf("reading the owner links: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var links []OwnerLink

	for rows.Next() {
		var (
			owner, kind    string
			slackID, label sql.NullString
		)

		err = rows.Scan(&owner, &kind, &slackID, &label)
		if err != nil {
			return nil, fmt.Errorf("reading an owner link: %w", err)
		}

		link, valid := ownerLinkFrom(owner, kind, slackID, label)
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

// ownerLinkFrom validates one row read back: an owner of the wrong shape or
// kind, or a link to a Slack ID of the wrong shape or kind for it, or to no
// Slack entity, is no link at all.
func ownerLinkFrom(owner, kind string, slackID, label sql.NullString) (OwnerLink, bool) {
	team := kind == kindTeam
	linkedWell := isSlackIDFor(team, slackID.String) && label.Valid

	if !isOwnerOfKind(owner, kind) || slackID.Valid && !linkedWell {
		return OwnerLink{}, false
	}

	return OwnerLink{
		Owner:   strings.ToLower(owner),
		Team:    team,
		OnSlack: slackID.Valid,
		Slack:   SlackTarget{ID: slackID.String, Label: cleanLabel(label.String)},
	}, true
}

// isOwnerOfKind reports an owner of the shape its kind takes: a person's
// name, or a team's, which a slash may divide.
func isOwnerOfKind(owner, kind string) bool {
	switch kind {
	case kindUser:
		return isOwner(owner) && !strings.Contains(owner, "/")
	case kindTeam:
		return isOwner(owner)
	default:
		return false
	}
}

// ownerIn is one forge owner on a host, as a Slack workspace sees them.
type ownerIn struct {
	forgeHost string
	workspace string
	owner     string
}

// LinkOwner records what was decided for a forge owner on a host in a Slack
// workspace, and whether they are a person or a team, as the caller tells:
// whom they are on Slack there, or that they are not on Slack there, replacing
// only that workspace's decision. It refuses an owner of the wrong shape for
// its kind with ErrInvalidOwner, a Slack ID of the wrong shape or kind — a
// user group for a team, a user for a person — with ErrInvalidSlackID, and a
// decision under no workspace with ErrNoWorkspace. The owner is kept in lower case, since
// both forges read user and team names without regard to case. A disabled or
// read-only store records nothing.
func (s Store) LinkOwner(ctx context.Context, forgeHost, workspace string, decision OwnerLink, now time.Time) error {
	switch {
	case forgeHost == "":
		return nil
	case !isOwnerOfKind(decision.Owner, kindOf(decision.Team)):
		return ErrInvalidOwner
	case decision.OnSlack && !isSlackIDFor(decision.Team, decision.Slack.ID):
		return ErrInvalidSlackID
	case workspace == "":
		return ErrNoWorkspace
	}

	where := ownerIn{forgeHost: forgeHost, workspace: workspace, owner: strings.ToLower(decision.Owner)}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		err := writeOwnerLink(ctx, transaction, where, decision, now)
		if err != nil {
			return err
		}

		return pruneSlackEntities(ctx, transaction)
	})
}

// ForgetOwner drops what was decided for a forge owner on a host in a Slack
// workspace, whatever the case of owner, so they are asked again there. What
// was decided in other workspaces is kept, and so then is the decision. A
// disabled or read-only store forgets nothing.
func (s Store) ForgetOwner(ctx context.Context, forgeHost, workspace, owner string) error {
	if forgeHost == "" {
		return nil
	}

	if workspace == "" {
		return ErrNoWorkspace
	}

	where := ownerIn{forgeHost: forgeHost, workspace: workspace, owner: strings.ToLower(owner)}

	return s.keptWithin(ctx, func(transaction *sql.Tx) error {
		err := unlinkIn(ctx, transaction, where)
		if err != nil {
			return err
		}

		_, err = transaction.ExecContext(ctx,
			`DELETE FROM owner_decision WHERE forge_host = ? AND owner = ?
				AND NOT EXISTS (SELECT 1 FROM owner_slack WHERE forge_host = ? AND owner = ?)
				AND NOT EXISTS (SELECT 1 FROM owner_not_on_slack WHERE forge_host = ? AND owner = ?)`,
			where.forgeHost, where.owner, where.forgeHost, where.owner, where.forgeHost, where.owner)
		if err != nil {
			return fmt.Errorf("forgetting the owner: %w", err)
		}

		return pruneSlackEntities(ctx, transaction)
	})
}

// kindOf is the kind an owner is kept as.
func kindOf(team bool) string {
	return map[bool]string{false: kindUser, true: kindTeam}[team]
}

// writeOwnerLink upserts the decision and replaces what was decided in the
// workspace: a Slack link, or that the owner is not on Slack there.
func writeOwnerLink(ctx context.Context, transaction *sql.Tx, where ownerIn, decision OwnerLink, now time.Time) error {
	_, err := transaction.ExecContext(ctx,
		`INSERT INTO owner_decision (forge_host, owner, decided_at, owner_kind) VALUES (?, ?, ?, ?)
			ON CONFLICT(forge_host, owner) DO UPDATE
				SET decided_at = excluded.decided_at, owner_kind = excluded.owner_kind`,
		where.forgeHost, where.owner, timestamp(now), kindOf(decision.Team))
	if err != nil {
		return fmt.Errorf("recording the owner decision: %w", err)
	}

	err = unlinkIn(ctx, transaction, where)
	if err != nil {
		return err
	}

	if !decision.OnSlack {
		_, err = transaction.ExecContext(ctx,
			`INSERT INTO owner_not_on_slack (forge_host, owner, slack_team) VALUES (?, ?, ?)`,
			where.forgeHost, where.owner, where.workspace)

		return wrapIfFailed("recording the owner not on Slack", err)
	}

	err = keepSlackEntity(ctx, transaction, decision.Slack, now)
	if err != nil {
		return err
	}

	_, err = transaction.ExecContext(ctx,
		`INSERT INTO owner_slack (forge_host, owner, slack_team, slack_id) VALUES (?, ?, ?, ?)`,
		where.forgeHost, where.owner, where.workspace, decision.Slack.ID)

	return wrapIfFailed("linking the owner to Slack", err)
}

// unlinkIn drops what was decided for the owner in the workspace: their Slack
// link, or that they are not on Slack there.
func unlinkIn(ctx context.Context, transaction *sql.Tx, where ownerIn) error {
	for _, statement := range []string{
		`DELETE FROM owner_slack WHERE forge_host = ? AND owner = ? AND slack_team = ?`,
		`DELETE FROM owner_not_on_slack WHERE forge_host = ? AND owner = ? AND slack_team = ?`,
	} {
		_, err := transaction.ExecContext(ctx, statement, where.forgeHost, where.owner, where.workspace)
		if err != nil {
			return fmt.Errorf("clearing what was decided for the owner: %w", err)
		}
	}

	return nil
}

// keepSlackEntity upserts a Slack user or group with the label it was just
// seen with, sanitized on the way in as well as on the way out.
func keepSlackEntity(ctx context.Context, transaction *sql.Tx, target SlackTarget, now time.Time) error {
	_, err := transaction.ExecContext(ctx,
		`INSERT INTO slack_entity (slack_id, label, seen_at) VALUES (?, ?, ?)
			ON CONFLICT(slack_id) DO UPDATE SET label = excluded.label, seen_at = excluded.seen_at`,
		target.ID, cleanLabel(target.Label), timestamp(now))

	return wrapIfFailed("keeping the Slack entity", err)
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

// isSlackIDFor reports a Slack ID of the kind an owner links to: a user
// group's for a team, a user's for a person.
func isSlackIDFor(team bool, slackID string) bool {
	return isSlackID(slackID, map[bool]string{false: userIDPrefixes, true: groupIDPrefixes}[team])
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
