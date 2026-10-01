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

// pattern is a CODEOWNERS path pattern split into segments, each a path.Match
// glob or a globstar.
type pattern struct {
	segments []string
	// directoryOnly is a pattern with a trailing slash: it names a directory,
	// so it matches what is under one and never a file of that name.
	directoryOnly bool
	// coversContents is a pattern that, matching a directory, matches
	// everything under it. dir/* does not: it covers direct children only.
	coversContents bool
	// source is the pattern as GitLab normalizes it, which GitLab keys a
	// section's rules by; empty on GitHub.
	source string
}

// compile reads a raw pattern as dialect reads it.
func compile(raw string, dialect Dialect) (pattern, bool) {
	if dialect == GitLab {
		return compileGitLab(raw), true
	}

	return compileGitHub(raw)
}

// compileGitHub reads a raw pattern as GitHub does. A leading slash anchors it
// to the repository root, as does a slash anywhere but the end.
func compileGitHub(raw string) (pattern, bool) {
	trimmed, directoryOnly := strings.CutSuffix(raw, "/")
	trimmed, anchored := strings.CutPrefix(trimmed, "/")

	if strings.Contains(trimmed, "/") {
		anchored = true
	}

	segments := splitPath(trimmed)
	if len(segments) == 0 {
		return pattern{}, false
	}

	if !anchored {
		segments = append([]string{globstar}, segments...)
	}

	return pattern{
		segments:       segments,
		directoryOnly:  directoryOnly,
		coversContents: directoryOnly || segments[len(segments)-1] != "*",
	}, true
}

// compileGitLab reads a raw pattern as GitLab's normalize_pattern and
// File.fnmatch? with FNM_DOTMATCH | FNM_PATHNAME do: the whole path must
// match, so only a pattern ending in a slash, which GitLab extends with **/*,
// covers a directory's contents.
func compileGitLab(raw string) pattern {
	source := normalizeGitLab(raw)
	segments := splitPath(source)

	for index, glob := range segments {
		segments[index] = negatedClasses(glob)
	}

	// fnmatch reads ** as a globstar only before a slash; elsewhere it is *.
	if last := len(segments) - 1; segments[last] == globstar {
		segments[last] = "*"
	}

	return pattern{segments: segments, source: source}
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

// negatedClasses spells fnmatch's [!...] as path.Match's [^...].
func negatedClasses(glob string) string {
	var out strings.Builder

	escaped, inClass := false, false

	for index := 0; index < len(glob); index++ {
		character := glob[index]
		out.WriteByte(character)

		switch {
		case escaped:
			escaped = false
		case character == '\\':
			escaped = true
		case inClass:
			inClass = character != ']'
		case character == '[':
			inClass = true

			if strings.HasPrefix(glob[index+1:], "!") {
				out.WriteByte('^')

				index++
			}
		}
	}

	return out.String()
}

// splitPath is a slash-separated path's segments, without empty ones, so a
// leading, trailing or doubled slash changes nothing.
func splitPath(slashed string) []string {
	return strings.FieldsFunc(slashed, func(character rune) bool { return character == '/' })
}

// matches reports whether the pattern matches a path: the whole of it, or —
// when the pattern covers a directory's contents — a directory it is under.
func (p pattern) matches(segments []string) bool {
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
func (p pattern) matchedPrefixes(segments []string) []bool {
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
