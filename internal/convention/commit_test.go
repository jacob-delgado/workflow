// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

func TestACommitConventionFallsBackToTheBuiltInDefaults(t *testing.T) {
	t.Parallel()

	// Arrange
	// Nothing configured: empty types, a zero limit, a blank trailer.
	built := convention.NewCommitConvention(nil, 0, "")

	// Act
	types := built.Types()

	// Assert
	if len(types) < 2 || types[0] != featType || types[1] != fixType {
		t.Errorf("Types() = %v, want the built-in types", types)
	}

	if built.SubjectLimit() != convention.DefaultCommitConvention().SubjectLimit() {
		t.Errorf("SubjectLimit() = %d, want the default", built.SubjectLimit())
	}

	if got := built.RefsLine("PROJ-1"); got != "Refs: PROJ-1" {
		t.Errorf("RefsLine() = %q, want the default Refs trailer", got)
	}
}

func TestACommitConventionAllowsOnlyItsConfiguredTypes(t *testing.T) {
	t.Parallel()

	// Arrange
	// A team that commits only hotfixes and chores, and never featType.
	built := convention.NewCommitConvention([]string{hotfixType, choreType}, 0, "")

	// Act
	custom := built.Validate(convention.Subject{Type: hotfixType, Description: "patch the leak"})
	standard := built.Validate(convention.Subject{Type: featType, Description: "add a thing"})

	// Assert
	if custom != nil {
		t.Errorf("a configured type does not validate: %v", custom)
	}

	if !errors.Is(standard, convention.ErrUnknownType) {
		t.Errorf("Validate(feat) = %v, want it rejected outside the configured types", standard)
	}
}

func TestACommitConventionMeasuresAgainstItsSubjectLimit(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		limit       int
		description string
		wantTooLong bool
	}{
		// "feat: " is six characters, so the subject is six longer than its body.
		"within a tighter limit":  {limit: 40, description: strings.Repeat("a", 34), wantTooLong: false},
		"past a tighter limit":    {limit: 40, description: strings.Repeat("a", 35), wantTooLong: true},
		"within the raised limit": {limit: 100, description: strings.Repeat("a", 90), wantTooLong: false},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			built := convention.NewCommitConvention(nil, testCase.limit, "")
			subject := convention.Subject{Type: featType, Description: testCase.description}

			// Act
			err := built.Validate(subject)

			// Assert
			if tooLong := errors.Is(err, convention.ErrSubjectTooLong); tooLong != testCase.wantTooLong {
				t.Errorf("Validate() = %v, want too-long %v at limit %d", err, testCase.wantTooLong, testCase.limit)
			}
		})
	}
}

func TestACommitConventionTrimsConfiguredTypes(t *testing.T) {
	t.Parallel()

	// Arrange
	// A padded type is accepted at load because the validator trims; the convention
	// must store it trimmed too, or it would never match a real subject.
	built := convention.NewCommitConvention([]string{" " + hotfixType + " ", choreType}, 0, "")

	// Act
	err := built.Validate(convention.Subject{Type: hotfixType, Description: "patch the leak"})
	// Assert
	if err != nil {
		t.Errorf("Validate(%q) = %v, want a padded configured type to match", hotfixType, err)
	}

	if got := built.Types(); got[0] != hotfixType {
		t.Errorf("Types()[0] = %q, want it trimmed to %q", got[0], hotfixType)
	}
}

func TestACommitConventionLabelsTheIssueTrailer(t *testing.T) {
	t.Parallel()

	// Arrange
	built := convention.NewCommitConvention(nil, 0, "Closes")
	subject := convention.Subject{Type: fixType, Description: "redact tokens"}

	// Act
	message := built.Message(subject, "", "PROJ-1")

	// Assert
	if !strings.Contains(message, "Closes: PROJ-1") || strings.Contains(message, "Refs: PROJ-1") {
		t.Errorf("Message() = %q, want the configured Closes trailer, not Refs", message)
	}
}

func TestACommitConventionReadsAConfiguredBranchType(t *testing.T) {
	t.Parallel()

	// Arrange
	// hotfixType is a configured type, so a hotfix/ branch opens on it.
	built := convention.NewCommitConvention([]string{hotfixType, choreType}, 0, "")

	// Act
	kind, ok := built.BranchType("hotfix/PROJ-1-leak")
	feat, isFeat := built.BranchType("feat/PROJ-1-thing")

	// Assert
	if !ok || kind != hotfixType {
		t.Errorf("BranchType(hotfix/…) = %q, %v, want hotfix", kind, ok)
	}

	if isFeat {
		t.Errorf("BranchType(feat/…) = %q, %v, want no match outside the configured types", feat, isFeat)
	}
}

func TestValidateTypeAcceptsALowercaseWord(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		value   string
		wantErr bool
	}{
		"a plain word":        {value: featType, wantErr: false},
		"a word with a digit": {value: "p2fix", wantErr: false},
		"empty":               {value: "", wantErr: true},
		"capitalized":         {value: "Feat", wantErr: true},
		"with a separator":    {value: "hot-fix", wantErr: true},
		"with a space":        {value: "hot fix", wantErr: true},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := convention.ValidateType(testCase.value)

			// Assert
			if gotErr := errors.Is(err, convention.ErrInvalidType); gotErr != testCase.wantErr {
				t.Errorf("ValidateType(%q) = %v, want error %v", testCase.value, err, testCase.wantErr)
			}
		})
	}
}
