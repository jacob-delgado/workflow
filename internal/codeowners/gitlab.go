// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package codeowners

import (
	"slices"
	"strings"
	"unicode"
)

// rubySpace is Ruby's \s, spelled out because Go's leaves out \v.
const rubySpace = `\t\n\v\f\r `

// gitLabHeader is GitLab's SectionParser::HEADER_REGEX: an optional ^, a name
// up to the first ], approvals only straight after it and only digits and
// whitespace, then default owners as far as a run of @, word characters, '.',
// '-', '/' and whitespace reaches. Nothing need follow: any line it matches
// is a header, whatever is after.
const gitLabHeader = `^(\^)?\[(.*?)\](?:\[[` + rubySpace + `\d]*\])?([` + rubySpace + `]*[@\w.\-/` + rubySpace + `]*)`

// rubyStrip is what Ruby's String#strip takes off both ends of a line.
const rubyStrip = "\x00\t\n\v\f\r "

// readGitLab reads a line as GitLab's CodeOwners::File does: stripped, then
// skipped when blank or a comment, a section header, skipped when it starts
// like a header GitLab cannot parse, or else an entry.
func (p *parser) readGitLab(line string) {
	line = strings.Trim(line, rubyStrip)
	if strings.TrimFunc(line, unicode.IsSpace) == "" || strings.HasPrefix(line, "#") {
		return
	}

	if match := p.header.FindStringSubmatch(line); match != nil {
		p.enterSection(match[2], strings.Trim(match[3], rubyStrip))

		return
	}

	if strings.HasPrefix(line, "[") || strings.HasPrefix(line, "^[") {
		return
	}

	p.addRule(fields(line))
}

// defaultSection is GitLab's Section::DEFAULT, the name of the section a file
// starts in. GitHub's one section takes it too, and nothing reads it there.
const defaultSection = "codeowners"

// enterSection makes name the current section, with the default owners its
// header names. A section named again is the same section.
func (p *parser) enterSection(name, defaults string) {
	index := p.sectionNamed(name)
	if index < 0 {
		p.file.sections = append(p.file.sections, section{name: name, rules: nil})
		index = len(p.file.sections) - 1
	}

	p.current, p.defaults = index, ownersFrom(fields(defaults))
}

// sectionNamed is the section a header names, or -1 for a new one, as
// GitLab's find_section_name finds it: while no section but the default
// exists, only the exact name codeowners is it; after, the first section of
// the name without case.
func (p *parser) sectionNamed(name string) int {
	if len(p.file.sections) == 1 {
		return slices.IndexFunc(p.file.sections, func(known section) bool { return known.name == name })
	}

	return slices.IndexFunc(p.file.sections, func(known section) bool { return strings.EqualFold(known.name, name) })
}
