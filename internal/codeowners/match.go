// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package codeowners

import (
	"path"
	"strings"
)

// globstar is the segment that matches any number of path segments, none
// included.
const globstar = "**"

// pattern is a CODEOWNERS path pattern compiled in one dialect: the pattern as
// that dialect normalizes it, which GitLab keys a section's rules by, and how
// the dialect matches a path against it.
type pattern struct {
	normalized string
	matches    func(candidate target) bool
}

// githubPattern is a pattern as GitHub reads it: split into segments, each a
// path.Match glob or a globstar.
type githubPattern struct {
	segments []string
	// directoryOnly is a pattern with a trailing slash: it names a directory,
	// so it matches what is under one and never a file of that name.
	directoryOnly bool
	// coversContents is a pattern that, matching a directory, matches
	// everything under it. dir/* does not: it covers direct children only.
	coversContents bool
}

// gitlabPattern is a pattern as GitLab's normalize_pattern leaves it, which
// GitLab matches against the whole path.
type gitlabPattern string

// target is a path to match, both as GitLab matches it, rooted at a slash,
// and as GitHub does, split into segments.
type target struct {
	rooted   string
	segments []string
}

func newTarget(path string) target {
	rooted := path
	if !strings.HasPrefix(rooted, "/") {
		rooted = "/" + rooted
	}

	return target{rooted: rooted, segments: splitPath(path)}
}

// compile reads a raw pattern as dialect reads it.
func compile(raw string, dialect Dialect) (pattern, bool) {
	if dialect == GitLab {
		normalized := normalizeGitLab(raw)

		return pattern{normalized: normalized, matches: gitlabPattern(normalized).matches}, true
	}

	compiled, ok := compileGitHub(raw)

	return pattern{normalized: raw, matches: compiled.matches}, ok
}

// compileGitHub reads a raw pattern as GitHub does. A leading slash anchors it
// to the repository root, as does a slash anywhere but the end.
func compileGitHub(raw string) (githubPattern, bool) {
	trimmed, directoryOnly := strings.CutSuffix(raw, "/")
	trimmed, anchored := strings.CutPrefix(trimmed, "/")

	if strings.Contains(trimmed, "/") {
		anchored = true
	}

	segments := splitPath(trimmed)
	if len(segments) == 0 {
		return githubPattern{}, false
	}

	if !anchored {
		segments = append([]string{globstar}, segments...)
	}

	return githubPattern{
		segments:       segments,
		directoryOnly:  directoryOnly,
		coversContents: directoryOnly || segments[len(segments)-1] != "*",
	}, true
}

// normalizeGitLab is GitLab's normalize_pattern: * is everything, an escaped
// leading # and escaped whitespace lose their backslash, an unanchored pattern
// matches at any depth, and a trailing slash covers what is under it.
func normalizeGitLab(raw string) string {
	if raw == "*" {
		return "/**/*"
	}

	if rest, found := strings.CutPrefix(raw, `\#`); found {
		raw = "#" + rest
	}

	raw = strings.NewReplacer(`\ `, " ", "\\\t", " ", "\\\r", " ", "\\\v", " ", "\\\f", " ").Replace(raw)

	if !strings.HasPrefix(raw, "/") {
		raw = "/**/" + raw
	}

	if strings.HasSuffix(raw, "/") {
		raw += "**/*"
	}

	return raw
}

// splitPath is a slash-separated path's segments, without empty ones, so a
// leading, trailing or doubled slash changes nothing.
func splitPath(slashed string) []string {
	return strings.FieldsFunc(slashed, func(character rune) bool { return character == '/' })
}

// matches reports whether the pattern matches the whole of a path rooted at a
// slash, as GitLab's File.fnmatch? does: only a pattern ending in a slash,
// which normalizeGitLab extends with **/*, covers a directory's contents.
func (p gitlabPattern) matches(candidate target) bool {
	return fnmatch(string(p), candidate.rooted)
}

// matches reports whether the pattern matches the whole of a path, or — when
// it covers a directory's contents — a directory the path is under.
func (p githubPattern) matches(candidate target) bool {
	segments := candidate.segments
	prefixes := p.matchedPrefixes(segments)
	whole := len(segments)

	if !p.directoryOnly && prefixes[whole] {
		return true
	}

	if !p.coversContents {
		return false
	}

	for length := 1; length < whole; length++ {
		if prefixes[length] {
			return true
		}
	}

	return false
}

// matchedPrefixes reports, for each length, whether the pattern matches the
// path's first that many segments. It walks the pattern once over the path,
// so a pattern full of globstars stays linear in each.
func (p githubPattern) matchedPrefixes(segments []string) []bool {
	reached := make([]bool, len(segments)+1)
	reached[0] = true

	for _, glob := range p.segments {
		next := make([]bool, len(segments)+1)

		if glob == globstar {
			sofar := false
			for length := range next {
				sofar = sofar || reached[length]
				next[length] = sofar
			}
		} else {
			for length := 1; length < len(next); length++ {
				next[length] = reached[length-1] && segmentMatches(glob, segments[length-1])
			}
		}

		reached = next
	}

	return reached
}

// segmentMatches reports whether one glob segment matches one path segment. A
// malformed glob matches nothing, as a forge would treat it.
func segmentMatches(glob, name string) bool {
	matched, err := path.Match(glob, name)

	return err == nil && matched
}
