// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"regexp"
	"slices"
	"strings"
)

// orderByClause finds an ORDER BY whatever its case and the whitespace around
// it, at the very start of a query too.
func orderByClause() *regexp.Regexp {
	return regexp.MustCompile(`(?i)(?:^|\s)order\s+by\s`)
}

// ScopedToMe narrows a view's JQL to the issues assigned to whoever the
// credential belongs to, unless the query already speaks of the assignee — a
// triage view of unassigned issues stays what it is. An ORDER BY stays last;
// one inside a quoted string or a parenthesis is the string's or the
// function's, not the query's. A query whose quotes or parentheses do not
// balance is left as it is too: where its ORDER BY begins cannot be told, and
// Jira refuses it whatever is added.
func ScopedToMe(jql string) string {
	if strings.Contains(strings.ToLower(jql), "assignee") {
		return jql
	}

	where, order, split := cutOrderBy(jql)
	if !split {
		return jql
	}

	scoped := "assignee = currentUser()"
	if where = strings.TrimSpace(where); where != "" {
		scoped = "(" + where + ") AND " + scoped
	}

	if order != "" {
		scoped += " ORDER BY " + order
	}

	return scoped
}

// cutOrderBy splits a query at its last ORDER BY outside every quoted string
// and parenthesis, whatever its case, and reports false when the query's
// quotes or parentheses do not balance.
func cutOrderBy(jql string) (string, string, bool) {
	outside, balanced := topLevel(jql)
	if !balanced {
		return "", "", false
	}

	found := orderByClause().FindAllStringIndex(outside, -1)
	if len(found) == 0 {
		return jql, "", true
	}

	last := found[len(found)-1]

	return jql[:last[0]], strings.TrimSpace(jql[last[1]:]), true
}

// topLevel is jql with every byte inside a quoted string or a parenthesis —
// the quotes and parentheses too — made an underscore, so what is left is the
// query's own words at the offsets jql has them; and whether every string and
// parenthesis closed, in order.
func topLevel(jql string) (string, bool) {
	var scan jqlScan

	words := []byte(jql)
	for index, char := range words {
		if !scan.outside(char) {
			words[index] = '_'
		}
	}

	return string(words), scan.balanced()
}

// jqlScan follows a query a byte at a time: the quote that closes the string
// it is in, if any, and whether the next byte is escaped; how many
// parentheses are open; and whether one ever closed before it opened.
type jqlScan struct {
	quote    byte
	escaped  bool
	depth    int
	overshot bool
}

// outside advances past char and reports whether it sits outside every quoted
// string and parenthesis.
func (s *jqlScan) outside(char byte) bool {
	if s.quoted(char) {
		return false
	}

	switch char {
	case '"', '\'':
		s.quote = char
	case '(':
		s.depth++
	case ')':
		s.depth--
		s.overshot = s.overshot || s.depth < 0
	default:
		return s.depth == 0
	}

	return false
}

// quoted advances past char when it is inside a quoted string, a backslash
// escaping the byte after it, and reports whether it was.
func (s *jqlScan) quoted(char byte) bool {
	switch {
	case s.quote == 0:
		return false
	case s.escaped:
		s.escaped = false
	case char == '\\':
		s.escaped = true
	case char == s.quote:
		s.quote = 0
	}

	return true
}

// balanced reports a query whose every string and parenthesis closed, in
// order.
func (s *jqlScan) balanced() bool {
	return s.quote == 0 && s.depth == 0 && !s.overshot
}

// KeysAssignedToMe is the JQL for which of keys name open issues assigned to
// whoever the credential belongs to: `key in (A, B) AND assignee =
// currentUser() AND statusCategory != done`, each key named once, in sorted
// order. A done issue stays assigned, and its branch lingers on the remote
// after the loop finishes it, so it is left out as the forge's open issues are.
func KeysAssignedToMe(keys []Key) string {
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, string(key))
	}

	slices.Sort(names)

	return "key in (" + strings.Join(slices.Compact(names), ", ") +
		") AND assignee = currentUser() AND statusCategory != done"
}
