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
}

// compile reads a raw pattern. A leading slash anchors it to the repository
// root; otherwise GitHub anchors it when it holds a slash anywhere but the end,
// and GitLab never does, reading an unanchored p as **/p.
func compile(raw string, dialect Dialect) (pattern, bool) {
	trimmed, directoryOnly := strings.CutSuffix(raw, "/")
	trimmed, anchored := strings.CutPrefix(trimmed, "/")

	if dialect == GitHub && strings.Contains(trimmed, "/") {
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
