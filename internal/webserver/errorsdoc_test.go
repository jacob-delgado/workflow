// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"os"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"

	apispec "github.com/jacob-delgado/workflow/api"
	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
)

// errorsPage is the reference page every problem's type URI points into.
const errorsPage = "../../docs/content/docs/errors.md"

func TestEveryProblemCodeHasASectionOnTheErrorsPage(t *testing.T) {
	t.Parallel()

	// Arrange
	codes := problemCodes(t)

	// Act
	anchors := sectionAnchors(t, errorsPage)

	// Assert
	for _, code := range codes {
		fragment := problemFragment(code)
		if !slices.Contains(anchors, fragment) {
			t.Errorf("problem code %q points at #%s, but %s has no section with that anchor; its sections are %q",
				code, fragment, errorsPage, anchors)
		}
	}
}

func TestAProblemsTypeEndsInItsCodesAnchor(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, serve(t, filledDeps(), config.Default()), "/api/issues?view=nope")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if want := "#" + problemFragment(string(api.NotFound)); !strings.HasSuffix(failure.Type, want) {
		t.Errorf("type = %q, want it to end in %q", failure.Type, want)
	}
}

// problemFragment is the anchor a problem's type URI ends in: its code with
// hyphens for underscores. TestAProblemsTypeEndsInItsCodesAnchor holds the
// server to it, so the errors page is checked against the anchors it sends.
func problemFragment(code string) string {
	return strings.ReplaceAll(code, "_", "-")
}

// problemCodes reads the Problem schema's code enum from the embedded contract.
func problemCodes(t *testing.T) []string {
	t.Helper()

	doc, err := openapi3.NewLoader().LoadFromData(apispec.Spec)
	if err != nil {
		t.Fatalf("load the contract: %v", err)
	}

	schema, found := doc.Components.Schemas["Problem"]
	if !found {
		t.Fatal("the contract has no Problem schema")
	}

	code, found := schema.Value.Properties["code"]
	if !found {
		t.Fatal("the Problem schema has no code property")
	}

	codes := make([]string, 0, len(code.Value.Enum))
	for _, value := range code.Value.Enum {
		name, isString := value.(string)
		if !isString {
			t.Fatalf("problem code %#v is a %T, not a string", value, value)
		}

		codes = append(codes, name)
	}

	if len(codes) == 0 {
		t.Fatal("the Problem schema's code enum is empty")
	}

	return codes
}

// sectionAnchors reads a page's second-level headings as the anchors Hugo
// generates for them.
func sectionAnchors(t *testing.T, path string) []string {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var anchors []string

	for line := range strings.Lines(string(contents)) {
		heading, isSection := strings.CutPrefix(line, "## ")
		if isSection {
			anchors = append(anchors, hugoAnchor(heading))
		}
	}

	return anchors
}

// hugoAnchor is the id Hugo's default "github" heading style gives a heading:
// lower case, spaces as hyphens, and every other character but a letter,
// digit, hyphen or underscore dropped.
func hugoAnchor(heading string) string {
	var anchor strings.Builder

	for _, char := range strings.ToLower(strings.TrimSpace(heading)) {
		switch {
		case char == ' ':
			anchor.WriteRune('-')
		case unicode.IsLetter(char), unicode.IsDigit(char), char == '-', char == '_':
			anchor.WriteRune(char)
		}
	}

	return anchor.String()
}
