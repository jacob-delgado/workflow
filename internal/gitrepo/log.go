// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package gitrepo

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// ErrNoIdentity is a repository with no user.email to tell your commits by.
var ErrNoIdentity = errors.New("git has no user.email to tell your commits by; set it with git config user.email")

// datedLogFormat is one commit as CommitsBetween reads it: the full hash, the
// short one, the author date and the subject, each ended by a NUL under -z.
const datedLogFormat = "--format=%H%x00%h%x00%aI%x00%s"

// datedFields is how many fields datedLogFormat writes for each commit.
const datedFields = 4

// DatedCommit is a commit you wrote, with when you wrote it.
type DatedCommit struct {
	Hash     string
	Short    string
	Subject  string
	Authored time.Time
}

// CommitsBetween reads the commits you wrote from start up to end,
// on any branch, merges and stashes left out. They are told by the email git commits
// under, and a repository with none is refused rather than read as everyone's.
// git can filter only by when a commit was made, which a rebase moves later,
// so every commit made since start is read and those written outside the
// period are left out by their author date. Every subject is sanitized,
// because a commit is anyone's to write.
func (r Repository) CommitsBetween(ctx context.Context, start, end time.Time) ([]DatedCommit, error) {
	email := optional(ctx, r.run, "-C", r.dir, "config", "user.email")
	if email == "" {
		return nil, ErrNoIdentity
	}

	// --fixed-strings, since git reads --author as a basic regular expression
	// in which no escaping is portable; a stash is not a commit you made.
	out, err := r.run(ctx, gitProgram, "-C", r.dir, "log", "-z", datedLogFormat,
		"--branches", "--remotes", "--tags", "--no-merges", "--fixed-strings",
		"--since="+start.UTC().Format(time.RFC3339), "--author=<"+email+">")
	if err != nil {
		return nil, readFailure(ctx, r.run, r.dir, "reading your commits", err)
	}

	var commits []DatedCommit

	for _, commit := range parseDatedLog(text(out)) {
		if !commit.Authored.Before(start) && commit.Authored.Before(end) {
			commits = append(commits, commit)
		}
	}

	return commits, nil
}

// parseDatedLog reads the commits datedLogFormat wrote, skipping one whose
// date git did not write as an instant.
func parseDatedLog(out string) []DatedCommit {
	fields := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")

	var commits []DatedCommit

	for start := 0; start+datedFields <= len(fields); start += datedFields {
		authored, err := time.Parse(time.RFC3339, fields[start+2])
		if err != nil {
			continue
		}

		commits = append(commits, DatedCommit{
			Hash: fields[start], Short: fields[start+1], Subject: sanitize.Line(fields[start+3]), Authored: authored.UTC(),
		})
	}

	return commits
}

// recentSubjectLimit bounds how far back RecentSubjects reads, enough to see the
// scopes a repository actually uses without walking its whole history.
const recentSubjectLimit = 200

// RecentSubjects reads the subjects of the repository's most recent commits,
// across every author, so their Conventional Commit scopes can be offered as
// completions. Each subject is sanitized, because a commit is anyone's to write.
func (r Repository) RecentSubjects(ctx context.Context) ([]string, error) {
	out, err := r.run(ctx, gitProgram, "-C", r.dir,
		"log", "--format=%s", "-n", strconv.Itoa(recentSubjectLimit))
	if err != nil {
		return nil, readFailure(ctx, r.run, r.dir, "reading recent subjects", err)
	}

	var subjects []string

	for line := range strings.SplitSeq(text(out), "\n") {
		if subject := sanitize.Text(line); subject != "" {
			subjects = append(subjects, subject)
		}
	}

	return subjects, nil
}
