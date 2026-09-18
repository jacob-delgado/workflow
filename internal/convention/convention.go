// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package convention holds the naming rules the workflow follows: what a branch
// for an issue is called, how an issue is found again from a branch, and what a
// Conventional Commit subject looks like.
//
// It is pure — no git, no network — because these are the decisions most worth
// testing exhaustively and cheapest to test that way.
package convention

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// SubjectLimit is the longest a commit subject may be, counted in characters.
// Past it, git's own tools and every forge's commit list truncate it.
const SubjectLimit = 72

// slugLimit caps the part of a branch name taken from the summary. A branch
// name is typed, tab-completed and read in narrow columns; the key already
// identifies the issue.
const slugLimit = 48

// Errors this package returns. Callers distinguish them with errors.Is.
var (
	// ErrInvalidBranchName reports a name git would refuse as a branch.
	ErrInvalidBranchName = errors.New("not a valid branch name")
	// ErrUnknownType reports a commit type outside CommitTypes.
	ErrUnknownType = errors.New("not a Conventional Commit type")
	// ErrInvalidScope reports a scope with characters a scope should not have.
	ErrInvalidScope = errors.New("a scope is lowercase letters, digits and . _ / -")
	// ErrNoDescription reports a subject with nothing after the colon.
	ErrNoDescription = errors.New("the subject needs a description")
	// ErrTrailingPeriod reports a description ending in a period.
	ErrTrailingPeriod = errors.New("a subject does not end with a period")
	// ErrSubjectTooLong reports a subject past SubjectLimit.
	ErrSubjectTooLong = errors.New("the subject is too long")
)

// issueKey matches a Jira issue key as Jira writes it: an uppercase project key
// of at least two characters, a hyphen, and a number. Uppercase only, because
// that is what Jira's development panel links, and because a lowercase match
// would read "utf-8" in a branch name as an issue.
func issueKey() *regexp.Regexp {
	return regexp.MustCompile(`(?:^|[^A-Za-z0-9])([A-Z][A-Z0-9_]+-[1-9][0-9]*)(?:[^0-9]|$)`)
}

// scope is what a Conventional Commit scope may contain.
func scope() *regexp.Regexp {
	return regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
}

// ValidateScope reports what is wrong with a Conventional Commit scope, or nil
// when it is well-formed. An empty scope is well-formed: a scope is optional.
func ValidateScope(value string) error {
	if trimmed := strings.TrimSpace(value); trimmed != "" && !scope().MatchString(trimmed) {
		return fmt.Errorf("%w: %q", ErrInvalidScope, trimmed)
	}

	return nil
}

// CommitTypes are the Conventional Commit types, in the order a composer offers
// them: the two that make up most commits first.
func CommitTypes() []string {
	return []string{"feat", "fix", "docs", "refactor", "test", "perf", "build", "ci", "chore", "style", "revert"}
}

// BranchName proposes a branch for an issue with the built-in convention: fix/
// for a bug and feat/ for anything else, then the key as Jira writes it, then a
// slug of the summary. A caller with configured naming uses BranchNaming.Name.
func BranchName(issueType, key, summary string) string {
	return DefaultBranchNaming().Name(issueType, key, summary)
}

// BranchType is the Conventional Commit type a branch name begins with — the
// part before the first slash — when that part is one, so a composer can open on
// the kind of change the branch already declares.
func BranchType(branchName string) (string, bool) {
	prefix, _, found := strings.Cut(branchName, "/")
	if found && slices.Contains(CommitTypes(), prefix) {
		return prefix, true
	}

	return "", false
}

// slugOf reduces a summary to lowercase ASCII words joined by hyphens, cut at a
// word boundary once it would pass slugLimit.
func slugOf(summary string) string {
	words := strings.FieldsFunc(folded(summary), func(character rune) bool {
		return character > unicode.MaxASCII || (!unicode.IsLetter(character) && !unicode.IsDigit(character))
	})

	slug := ""

	for _, word := range words {
		next := strings.ToLower(word)
		if slug != "" {
			next = slug + "-" + next
		}

		if len(next) > slugLimit {
			break
		}

		slug = next
	}

	return slug
}

// folded strips accents, so "Café" slugs as "cafe" rather than losing the é.
func folded(text string) string {
	var plain strings.Builder

	for _, character := range norm.NFKD.String(text) {
		if !unicode.Is(unicode.Mn, character) {
			plain.WriteRune(character)
		}
	}

	return plain.String()
}

// IssueKey finds the issue key in text, such as a branch name: a Jira key like
// PROJ-42 where there is one, and otherwise a forge issue number like the 42 in
// a 42-fix-typo branch, which is how a project without Jira names its branches.
// A Jira key wins where both are present, so a Jira branch is read as it always
// was.
func IssueKey(text string) (string, bool) {
	if key, found := jiraKey(text); found {
		return key, true
	}

	return forgeKey(text)
}

// jiraKey finds the first Jira issue key in text.
func jiraKey(text string) (string, bool) {
	for _, match := range issueKey().FindAllStringSubmatch(text, -1) {
		project, _, _ := strings.Cut(match[1], "-")
		if !standardAbbreviations()[project] {
			return match[1], true
		}
	}

	return "", false
}

// forgeIssueKey matches a forge issue number as a branch names one: a number at
// the start of the branch or of a path component, ending the component or before
// a hyphen, as GitLab's own "42-fix-typo" branches are named. Anchoring it there
// keeps a slug digit, or a token like the 256 in SHA-256, from reading as a key.
func forgeIssueKey() *regexp.Regexp {
	return regexp.MustCompile(`(?:^|/)([1-9][0-9]*)(?:-|$)`)
}

// forgeKey finds a forge issue number in text.
func forgeKey(text string) (string, bool) {
	match := forgeIssueKey().FindStringSubmatch(text)
	if match == nil {
		return "", false
	}

	return match[1], true
}

// standardAbbreviations are common uppercase-and-number tokens shaped like a
// Jira key that are not one, so a branch or a title mentioning UTF-8, SHA-256 or
// CVE-2024 does not derive a phantom issue and skip a real key beside it.
func standardAbbreviations() map[string]bool {
	return map[string]bool{"UTF": true, "SHA": true, "CVE": true, "ISO": true, "RFC": true, "MD": true}
}

// ValidateBranchName reports why git would refuse a branch name, by the rules of
// git check-ref-format --branch, so a name can be corrected as it is typed
// rather than after git has said no.
func ValidateBranchName(name string) error {
	reason := branchNameProblem(name)
	if reason == "" {
		return nil
	}

	return fmt.Errorf("%w: %s", ErrInvalidBranchName, reason)
}

// branchNameProblem names what is wrong with a branch name, or returns "".
func branchNameProblem(name string) string {
	for _, check := range []func(string) string{shapeProblem, contentProblem} {
		if reason := check(name); reason != "" {
			return reason
		}
	}

	return componentProblem(strings.Split(name, "/"))
}

// shapeProblem checks how a branch name starts and ends.
func shapeProblem(name string) string {
	switch {
	case name == "":
		return "it is empty"
	case name == "@":
		return `it is "@", which git reads as the current branch`
	case strings.HasPrefix(name, "-"):
		return "it starts with a hyphen"
	case strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "//"):
		return "it has an empty part between slashes"
	case strings.HasSuffix(name, "."):
		return "it ends with a period"
	default:
		return ""
	}
}

// contentProblem checks for sequences and characters git refuses anywhere.
func contentProblem(name string) string {
	switch {
	case strings.Contains(name, "..") || strings.Contains(name, "@{"):
		return `it contains ".." or "@{"`
	case strings.ContainsFunc(name, forbiddenInBranch):
		return `it contains a space, a control character, or one of ~ ^ : ? * [ \`
	default:
		return ""
	}
}

// componentProblem checks each slash-separated part of a branch name.
func componentProblem(components []string) string {
	if slices.ContainsFunc(components, func(component string) bool { return strings.HasPrefix(component, ".") }) {
		return "a part starts with a period"
	}

	if slices.ContainsFunc(components, func(component string) bool { return strings.HasSuffix(component, ".lock") }) {
		return `a part ends with ".lock"`
	}

	return ""
}

// forbiddenInBranch reports a character git does not allow anywhere in a ref.
func forbiddenInBranch(character rune) bool {
	return unicode.IsControl(character) || strings.ContainsRune(" ~^:?*[\\", character)
}

// Subject is a Conventional Commit subject line, in parts.
type Subject struct {
	Type        string
	Scope       string
	Description string
	// Breaking marks a change people depending on it must react to, with "!".
	Breaking bool
}

// String assembles the subject line.
func (s Subject) String() string {
	line := strings.TrimSpace(s.Type)

	if part := strings.TrimSpace(s.Scope); part != "" {
		line += "(" + part + ")"
	}

	if s.Breaking {
		line += "!"
	}

	return line + ": " + strings.TrimSpace(s.Description)
}

// Validate reports the first thing wrong with the subject, in the order the
// parts are written.
func (s Subject) Validate() error {
	kind := strings.TrimSpace(s.Type)
	if !slices.Contains(CommitTypes(), kind) {
		return fmt.Errorf("%w: %q", ErrUnknownType, kind)
	}

	err := ValidateScope(s.Scope)
	if err != nil {
		return err
	}

	description := strings.TrimSpace(s.Description)

	switch {
	case description == "":
		return ErrNoDescription
	case strings.HasSuffix(description, "."):
		return ErrTrailingPeriod
	case utf8.RuneCountInString(s.String()) > SubjectLimit:
		return fmt.Errorf("%w: %d of %d characters", ErrSubjectTooLong, utf8.RuneCountInString(s.String()), SubjectLimit)
	default:
		return nil
	}
}

// Message is a whole commit message: the subject, the body if there is one, and
// a Refs trailer naming the issue, unless the body already carries it.
func Message(subject Subject, body, issueKey string) string {
	body = strings.TrimSpace(body)

	message := subject.String()
	if body != "" {
		message += "\n\n" + body
	}

	trailer := "Refs: " + issueKey
	if issueKey != "" && !hasTrailerLine(body, trailer) {
		message += trailerJoin(body) + trailer
	}

	return message + "\n"
}

// hasTrailerLine reports a trailer already present as a whole line, so a longer
// key that only contains this one — PROJ-412 inside PROJ-4120 — does not pass
// for it.
func hasTrailerLine(text, trailer string) bool {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) == trailer {
			return true
		}
	}

	return false
}

// trailerJoin is what separates an appended trailer from the body: a single
// newline when the body already ends in a trailer block — so the trailers stay
// one paragraph that git interpret-trailers reads together, rather than
// stranding a Co-authored-by in an earlier one — and a blank line otherwise.
func trailerJoin(body string) string {
	if body != "" && endsWithTrailerBlock(body) {
		return "\n"
	}

	return "\n\n"
}

// endsWithTrailerBlock reports whether the body's last paragraph is entirely
// trailer lines.
func endsWithTrailerBlock(body string) bool {
	paragraphs := strings.Split(body, "\n\n")
	last := paragraphs[len(paragraphs)-1]

	for line := range strings.SplitSeq(last, "\n") {
		if strings.TrimSpace(line) != "" && !trailerLine().MatchString(line) {
			return false
		}
	}

	return true
}

// trailerLine matches a git trailer: a key of letters, digits and hyphens, a
// colon, and a space.
func trailerLine() *regexp.Regexp {
	return regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*: `)
}
