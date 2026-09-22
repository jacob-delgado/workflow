// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention

import "strings"

// defaultTemplate is the branch shape used when none is configured: a
// Conventional Commit prefix, the issue key, and a slug of the summary.
const defaultTemplate = "{prefix}/{key}-{slug}"

// defaultFallbackPrefix is the prefix for an issue type with no mapping of its
// own — everything that is not a bug.
const defaultFallbackPrefix = "feat"

// BranchNaming produces a branch name for an issue from a template and a
// type-to-prefix map, so a team can shape names to its own convention while the
// issue key stays readable back out of the result.
type BranchNaming struct {
	template  string
	prefixes  map[string]string
	fallback  string
	slugLimit int
}

// DefaultBranchNaming is the built-in convention: fix/ for a bug and feat/ for
// anything else, then the key, then a slug of the summary.
func DefaultBranchNaming() BranchNaming {
	return BranchNaming{
		template:  defaultTemplate,
		prefixes:  map[string]string{"bug": "fix"},
		fallback:  defaultFallbackPrefix,
		slugLimit: defaultSlugLimit,
	}
}

// NewBranchNaming builds a naming from configured pieces, each falling back to
// the default when empty or zero. A configured prefix map replaces the built-in
// one rather than adding to it, so what a team writes down is the whole rule.
func NewBranchNaming(template, defaultPrefix string, prefixes map[string]string, slugLimit int) BranchNaming {
	naming := DefaultBranchNaming()

	if template != "" {
		naming.template = template
	}

	if defaultPrefix != "" {
		naming.fallback = defaultPrefix
	}

	if len(prefixes) > 0 {
		naming.prefixes = lowerKeys(prefixes)
	}

	if slugLimit > 0 {
		naming.slugLimit = slugLimit
	}

	return naming
}

// Name proposes a branch for an issue.
func (n BranchNaming) Name(issueType, key, summary string) string {
	name := n.template
	name = strings.ReplaceAll(name, "{prefix}", n.prefix(issueType))
	name = strings.ReplaceAll(name, "{key}", key)
	name = strings.ReplaceAll(name, "{slug}", slugOf(summary, n.slugLimit))

	return tidyBranchName(name)
}

// prefix is the branch prefix for an issue type, matched without regard to case.
func (n BranchNaming) prefix(issueType string) string {
	if mapped, ok := n.prefixes[strings.ToLower(strings.TrimSpace(issueType))]; ok {
		return mapped
	}

	return n.fallback
}

// tidyBranchName repairs the separators an empty placeholder leaves behind, so a
// summary that slugs to nothing does not strand a trailing or doubled hyphen.
func tidyBranchName(name string) string {
	return strings.TrimRight(strings.ReplaceAll(name, "--", "-"), "-/_")
}

// lowerKeys copies a prefix map with its keys folded to lower case, so a type
// written "Bug" in the file matches "bug" from Jira.
func lowerKeys(prefixes map[string]string) map[string]string {
	folded := make(map[string]string, len(prefixes))
	for issueType, prefix := range prefixes {
		folded[strings.ToLower(strings.TrimSpace(issueType))] = prefix
	}

	return folded
}
