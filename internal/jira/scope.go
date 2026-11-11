// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"regexp"
	"slices"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
)

// orderByClause matches an ORDER BY whatever its case and the whitespace around
// it, at the very start of a query too.
var orderByClause = regexp.MustCompile(`(?i)(?:^|\s)order\s+by\s`)

// assigneeIsMe is the clause that narrows a query to the issues assigned to
// whoever the credential belongs to.
const assigneeIsMe = "assignee = currentUser()"

// assigneeField matches the assignee field named whatever its case, as a word
// of its own.
var assigneeField = regexp.MustCompile(`(?i)\bassignee\b`)

// assigneeFieldName is the assignee field's name, which a query may also quote
// whole, as `"assignee" = bob`.
const assigneeFieldName = "assignee"

// DefaultViewName titles the one built-in view a configuration naming none
// gets: the open issues assigned to you.
const DefaultViewName = "Assigned to me"

// View is a named issue list and the JQL that fills it, as every surface
// lists it.
type View struct {
	Name string
	JQL  string
}

// Views are the views a configuration offers, each scoped to your issues
// unless its query names the assignee, or the one built-in view when it names
// none, so there is always a first. The JQL is the scoped query, which is what
// is searched and what the issue cache is keyed by.
func Views(configured []config.JiraView) []View {
	if len(configured) == 0 {
		return []View{{Name: DefaultViewName, JQL: AssignedToMe}}
	}

	views := make([]View, 0, len(configured))
	for _, view := range configured {
		views = append(views, View{Name: view.Name, JQL: ScopedToMe(view.JQL)})
	}

	return views
}

// ViewNamed is the view of that exact name, or the first view when the name is
// empty. It reports false for a name no view carries, so a typo is refused
// rather than quietly answered with another view's issues.
func ViewNamed(configured []config.JiraView, name string) (View, bool) {
	views := Views(configured)
	if name == "" {
		return views[0], true
	}

	for _, view := range views {
		if view.Name == name {
			return view, true
		}
	}

	return View{}, false
}

// ScopedToMe narrows a view's JQL to the issues assigned to whoever the
// credential belongs to, unless the query already speaks of the assignee — a
// triage view of unassigned issues stays what it is, and so does one that
// quotes the field's name whole, as `"assignee" = bob`. The word among other
// text in a quoted string, as text a summary is searched for or a field named
// "Assignee Group", does not count. An ORDER BY stays last; one inside a quoted string
// or a parenthesis is the string's or the function's, not the query's. A
// query whose quotes or parentheses do not balance is left as it is too:
// where its ORDER BY begins cannot be told, and Jira refuses it whatever is
// added.
func ScopedToMe(jql string) string {
	if namesAssignee(jql) {
		return jql
	}

	where, order, split := cutOrderBy(jql)
	if !split {
		return jql
	}

	scoped := assigneeIsMe
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

	found := orderByClause.FindAllStringIndex(outside, -1)
	if len(found) == 0 {
		return jql, "", true
	}

	last := found[len(found)-1]

	return jql[:last[0]], strings.TrimSpace(jql[last[1]:]), true
}

// namesAssignee reports a query that speaks of the assignee field: by its name
// as a word of the query's own, or in a quoted string that holds the name
// alone, whatever its case.
func namesAssignee(jql string) bool {
	if assigneeField.MatchString(unquoted(jql)) {
		return true
	}

	return slices.ContainsFunc(quotedStrings(jql), func(text string) bool {
		return strings.EqualFold(text, assigneeFieldName)
	})
}

// quotedStrings is the text of each quoted string jql closes, as written
// between its quotes.
func quotedStrings(jql string) []string {
	var (
		scan  jqlScan
		texts []string
		start int
	)

	for index := range len(jql) {
		wasQuoted := scan.quote != 0
		scan.place(jql[index])

		switch isQuoted := scan.quote != 0; {
		case !wasQuoted && isQuoted:
			start = index + 1
		case wasQuoted && !isQuoted:
			texts = append(texts, jql[start:index])
		}
	}

	return texts
}

// unquoted is jql with every byte of a quoted string, its quotes too, made an
// underscore, so what is left is the query's own words, a function's
// arguments and a parenthesized clause's among them.
func unquoted(jql string) string {
	words, _ := masked(jql, func(place jqlPlace) bool { return place == inString })

	return words
}

// topLevel is jql with every byte inside a quoted string or a parenthesis —
// the quotes and parentheses too — made an underscore, so what is left is the
// query's own words at the offsets jql has them; and whether every string and
// parenthesis closed, in order.
func topLevel(jql string) (string, bool) {
	return masked(jql, func(place jqlPlace) bool { return place != atTopLevel })
}

// masked is jql with every byte whose place hide picks made an underscore, at
// the offsets jql has them, and whether every string and parenthesis closed,
// in order.
func masked(jql string, hide func(jqlPlace) bool) (string, bool) {
	var scan jqlScan

	words := []byte(jql)
	for index, char := range words {
		if hide(scan.place(char)) {
			words[index] = '_'
		}
	}

	return string(words), scan.balanced()
}

// jqlPlace is where a byte of a query sits: in the query's own words, inside
// a parenthesis, or inside a quoted string; a quote or a parenthesis sits
// inside what it opens or closes.
type jqlPlace int

const (
	atTopLevel jqlPlace = iota
	inParentheses
	inString
)

// jqlScan follows a query a byte at a time: the quote that closes the string
// it is in, if any, and whether the next byte is escaped; how many
// parentheses are open; and whether one ever closed before it opened.
type jqlScan struct {
	quote    byte
	escaped  bool
	depth    int
	overshot bool
}

// place advances past char and reports where it sits.
func (s *jqlScan) place(char byte) jqlPlace {
	if s.quoted(char) {
		return inString
	}

	switch char {
	case '"', '\'':
		s.quote = char

		return inString
	case '(':
		s.depth++
	case ')':
		s.depth--
		s.overshot = s.overshot || s.depth < 0
	default:
		if s.depth == 0 {
			return atTopLevel
		}
	}

	return inParentheses
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

// NoKeys is the query a search for assigned keys is asked with when every key
// is a forge issue's number: no Jira key to name, and nothing for Jira to
// answer. A tracker that reads the query asks nothing for it; the forge's own
// issues, which ignore the query, answer it as any other.
const NoKeys = ""

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
		") AND " + assigneeIsMe + " AND statusCategory != done"
}
