// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Defaults a team inherits when it configures nothing.
const (
	defaultSubjectLimit = 72
	defaultRefsLabel    = "Refs"
)

// commitType is what a Conventional Commit type may contain: a lowercase word,
// as "feat" or "hotfix" — the same shape a branch prefix takes.
func commitType() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z][a-z0-9]*$`)
}

// ValidateType reports what is wrong with a configured commit type, or nil when
// it is well-formed — a lowercase word usable as a subject prefix and a branch
// segment.
func ValidateType(value string) error {
	if trimmed := strings.TrimSpace(value); trimmed == "" || !commitType().MatchString(trimmed) {
		return fmt.Errorf("%w: %q", ErrInvalidType, value)
	}

	return nil
}

// defaultCommitTypes are the Conventional Commit types, in the order a composer
// offers them: the two that make up most commits first.
func defaultCommitTypes() []string {
	return []string{"feat", "fix", "docs", "refactor", "test", "perf", "build", "ci", "chore", "style", "revert"}
}

// CommitConvention shapes the commit messages a team writes: which types are
// allowed and in what order they are offered, how long a subject may be, and how
// the issue trailer is labeled. It is built from configuration, each piece
// falling back to the built-in default.
type CommitConvention struct {
	types        []string
	subjectLimit int
	refsLabel    string
}

// DefaultCommitConvention is the built-in convention: the standard Conventional
// Commit types, a 72-character subject, and a "Refs" issue trailer.
func DefaultCommitConvention() CommitConvention {
	return CommitConvention{types: defaultCommitTypes(), subjectLimit: defaultSubjectLimit, refsLabel: defaultRefsLabel}
}

// NewCommitConvention builds a convention from configured pieces, each falling
// back to the default when empty or zero.
func NewCommitConvention(types []string, subjectLimit int, refsLabel string) CommitConvention {
	commit := DefaultCommitConvention()

	if len(types) > 0 {
		// Trim each type, as the load-time validator does before accepting it, so a
		// padded " feat " matches a real "feat" subject rather than never matching.
		commit.types = trimmedTypes(types)
	}

	if subjectLimit > 0 {
		commit.subjectLimit = subjectLimit
	}

	if trimmed := strings.TrimSpace(refsLabel); trimmed != "" {
		commit.refsLabel = trimmed
	}

	return commit
}

// trimmedTypes copies the types with surrounding whitespace removed, so a padded
// entry matches the trimmed subject type the composer and validator compare with.
func trimmedTypes(types []string) []string {
	trimmed := make([]string, 0, len(types))
	for _, commitType := range types {
		trimmed = append(trimmed, strings.TrimSpace(commitType))
	}

	return trimmed
}

// Types are the commit types this convention allows, in the order to offer them.
func (c CommitConvention) Types() []string {
	return slices.Clone(c.types)
}

// SubjectLimit is the longest a subject may be under this convention, counted in
// characters.
func (c CommitConvention) SubjectLimit() int {
	return c.subjectLimit
}

// RefsLine is the issue trailer this convention appends, "<label>: <issueKey>".
func (c CommitConvention) RefsLine(issueKey string) string {
	return c.refsLabel + ": " + issueKey
}

// BranchType is the commit type a branch name begins with — the part before the
// first slash — when that part is one this convention allows, so a composer can
// open on the kind of change the branch already declares.
func (c CommitConvention) BranchType(branchName string) (string, bool) {
	prefix, _, found := strings.Cut(branchName, "/")
	if found && slices.Contains(c.types, prefix) {
		return prefix, true
	}

	return "", false
}

// Validate reports the first thing wrong with a subject under this convention,
// in the order the parts are written.
func (c CommitConvention) Validate(subject Subject) error {
	kind := strings.TrimSpace(subject.Type)
	if !slices.Contains(c.types, kind) {
		return fmt.Errorf("%w: %q", ErrUnknownType, kind)
	}

	err := ValidateScope(subject.Scope)
	if err != nil {
		return err
	}

	description := strings.TrimSpace(subject.Description)

	switch {
	case description == "":
		return ErrNoDescription
	case strings.ContainsFunc(description, unicode.IsControl):
		return ErrSubjectNotOneLine
	case strings.HasSuffix(description, "."):
		return ErrTrailingPeriod
	case utf8.RuneCountInString(subject.String()) > c.subjectLimit:
		return fmt.Errorf("%w: %d of %d characters", ErrSubjectTooLong,
			utf8.RuneCountInString(subject.String()), c.subjectLimit)
	default:
		return nil
	}
}

// Message assembles a whole commit message under this convention: the subject,
// the body if there is one, and the issue trailer unless the body already
// carries it.
func (c CommitConvention) Message(subject Subject, body, issueKey string) string {
	body = strings.TrimSpace(body)

	message := subject.String()
	if body != "" {
		message += "\n\n" + body
	}

	trailer := c.RefsLine(issueKey)
	if issueKey != "" && !hasTrailerLine(body, trailer) {
		message += trailerJoin(body) + trailer
	}

	return message + "\n"
}
