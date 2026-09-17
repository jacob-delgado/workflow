// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package testshape

import (
	"go/ast"
	"go/token"
	"strings"
)

// markerKind is which part of a test a marker opens.
type markerKind int

const (
	arrange markerKind = iota
	act
	assert
	actAndAssert
)

// marker is a well-formed marker comment.
type marker struct {
	kind    markerKind
	labeled bool
	pos     token.Pos
}

// markerKeywords maps each marker, as written after "// ", to its kind.
func markerKeywords() map[string]markerKind {
	return map[string]markerKind{"Arrange": arrange, "Act": act, "Assert": assert, "Act & Assert": actAndAssert}
}

// looseKeywords are the markers once case, spacing and "and" are set aside,
// which is how a comment meant as a marker but written wrongly is recognized.
func looseKeywords() map[string]bool {
	return map[string]bool{"arrange": true, "act": true, "assert": true, "act & assert": true}
}

// comments are the markers found in a body, split by whether they are well
// formed.
type comments struct {
	markers   []marker
	malformed []token.Pos
}

// readComments finds the markers among the comments inside a body, skipping
// those inside any of the excluded ranges.
func (p *pkg) readComments(file *ast.File, body *ast.BlockStmt, excluded []*ast.FuncLit) comments {
	var found comments

	for _, group := range file.Comments {
		for _, comment := range group.List {
			if comment.Pos() <= body.Lbrace || comment.Pos() >= body.Rbrace || within(comment.Pos(), excluded) {
				continue
			}

			if parsed, ok := p.parseMarker(comment.Text); ok {
				parsed.pos = comment.Pos()
				found.markers = append(found.markers, parsed)
			} else if p.looksLikeMarker(comment.Text) {
				found.malformed = append(found.malformed, comment.Pos())
			}
		}
	}

	return found
}

// within reports whether pos is inside any of the literals.
func within(pos token.Pos, literals []*ast.FuncLit) bool {
	for _, literal := range literals {
		if pos >= literal.Pos() && pos < literal.End() {
			return true
		}
	}

	return false
}

// parseMarker reads a comment written exactly as a marker: "// " and a keyword,
// then optionally ": " and a label.
func (p *pkg) parseMarker(text string) (marker, bool) {
	rest, ok := strings.CutPrefix(strings.TrimRight(text, " \t"), "// ")
	if !ok {
		return marker{}, false
	}

	keyword, label, labeled := strings.Cut(rest, ":")

	kind, known := p.keywords[keyword]
	if !known {
		return marker{}, false
	}

	if labeled && (!strings.HasPrefix(label, " ") || strings.TrimSpace(label) == "") {
		return marker{}, false
	}

	return marker{kind: kind, labeled: labeled, pos: token.NoPos}, true
}

// looksLikeMarker reports whether a comment that is not a well-formed marker was
// meant to be one: the same keyword in another case, spacing or comment style.
// Prose that merely starts with a keyword is not.
func (p *pkg) looksLikeMarker(text string) bool {
	body := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(text, "//"), "/*"), "*/")
	loose := strings.ToLower(strings.Join(strings.Fields(body), " "))
	loose = strings.ReplaceAll(loose, " and ", " & ")
	keyword, _, _ := strings.Cut(loose, ":")

	return p.loose[strings.TrimSpace(keyword)]
}

// misplaced reports whether a marker is anywhere but on a line of its own
// between a body's top-level statements.
func (p *pkg) misplaced(body *ast.BlockStmt, pos token.Pos) bool {
	line := p.fset.Position(pos).Line

	if p.fset.Position(body.Lbrace).Line == line {
		return true
	}

	for _, statement := range body.List {
		inside := pos > statement.Pos() && pos < statement.End()
		sharesLine := p.fset.Position(statement.Pos()).Line == line || p.fset.Position(statement.End()).Line == line

		if inside || sharesLine {
			return true
		}
	}

	return false
}
