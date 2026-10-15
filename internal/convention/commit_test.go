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

	if got := built.RefsLabel(); got != "Refs" {
		t.Errorf("RefsLabel() = %q, want the default Refs", got)
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

func TestSubjectAssemblesAConventionalCommit(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		subject convention.Subject
		want    string
	}{
		"with a scope": {
			subject: convention.Subject{Type: fixType, Scope: "config", Description: redactTokens, Breaking: false},
			want:    redactSubject,
		},
		"without one": {
			subject: convention.Subject{Type: "docs", Scope: "", Description: "explain keys", Breaking: false},
			want:    "docs: explain keys",
		},
		"breaking": {
			subject: convention.Subject{Type: featType, Scope: "cli", Description: "drop --old", Breaking: true},
			want:    "feat(cli)!: drop --old",
		},
		"space around the parts is trimmed": {
			subject: convention.Subject{Type: " feat ", Scope: " tui ", Description: "  add panes ", Breaking: false},
			want:    "feat(tui): add panes",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.subject.String()

			// Assert
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}

			err := tt.subject.Validate()
			if err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
		})
	}
}

func TestValidateScopeAcceptsAScopeAndRejectsBadCharacters(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		scope   string
		wantErr bool
	}{
		"a plain scope":     {scope: "parser", wantErr: false},
		"with punctuation":  {scope: "api/v2_beta.1", wantErr: false},
		"empty is optional": {scope: "", wantErr: false},
		"a space":           {scope: "two words", wantErr: true},
		"capitalized":       {scope: "Config", wantErr: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			err := convention.ValidateScope(tt.scope)

			// Assert
			if (err != nil) != tt.wantErr || (err != nil && !errors.Is(err, convention.ErrInvalidScope)) {
				t.Errorf("ValidateScope(%q) = %v, want error: %t", tt.scope, err, tt.wantErr)
			}
		})
	}
}

func TestSubjectValidationSaysWhatIsWrong(t *testing.T) {
	t.Parallel()

	valid := convention.Subject{Type: fixType, Scope: "", Description: redactTokens, Breaking: false}
	describe := func(description string) func(*convention.Subject) {
		return func(s *convention.Subject) { s.Description = description }
	}

	cases := map[string]struct {
		change func(*convention.Subject)
		want   error
	}{
		"no type":        {change: func(s *convention.Subject) { s.Type = "" }, want: convention.ErrUnknownType},
		"an unknown one": {change: func(s *convention.Subject) { s.Type = "feature" }, want: convention.ErrUnknownType},
		"a scope with a space": {
			change: func(s *convention.Subject) { s.Scope = "two words" }, want: convention.ErrInvalidScope,
		},
		"a capitalized scope": {change: func(s *convention.Subject) { s.Scope = "Config" }, want: convention.ErrInvalidScope},
		"no description":      {change: describe("  "), want: convention.ErrNoDescription},
		"a trailing period":   {change: describe("redact tokens."), want: convention.ErrTrailingPeriod},
		"too long":            {change: describe(strings.Repeat("x", 68)), want: convention.ErrSubjectTooLong},
		"a line break":        {change: describe("a\nb"), want: convention.ErrSubjectNotOneLine},
		"a carriage return":   {change: describe("a\rb"), want: convention.ErrSubjectNotOneLine},
		"a tab":               {change: describe("a\tb"), want: convention.ErrSubjectNotOneLine},
		"an escape sequence":  {change: describe("a\x1b[2Jb"), want: convention.ErrSubjectNotOneLine},
		"a delete":            {change: describe("a\x7fb"), want: convention.ErrSubjectNotOneLine},
		"a C1 next line":      {change: describe("a\xc2\x85b"), want: convention.ErrSubjectNotOneLine},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			subject := valid
			tt.change(&subject)

			// Act
			err := subject.Validate()

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

// The limit counts the whole line, and exactly the limit is allowed.
func TestASubjectOfExactlyTheLimitIsAllowed(t *testing.T) {
	t.Parallel()

	// Arrange
	atLimit := convention.Subject{Type: fixType, Scope: "", Description: strings.Repeat("x", 67), Breaking: false}

	// Act
	err := atLimit.Validate()

	// Assert
	if err != nil || len(atLimit.String()) != convention.DefaultCommitConvention().SubjectLimit() {
		t.Errorf("a %d-character subject: Validate() = %v, want nil", len(atLimit.String()), err)
	}
}

func TestCommitTypesOfferTheCommonOnesFirst(t *testing.T) {
	t.Parallel()

	// Act
	types := convention.DefaultCommitConvention().Types()

	// Assert
	if len(types) < 2 || types[0] != "feat" || types[1] != "fix" {
		t.Errorf("Types() = %v, want feat and fix first", types)
	}

	for _, each := range types {
		subject := convention.Subject{Type: each, Scope: "", Description: "x", Breaking: false}

		err := subject.Validate()
		if err != nil {
			t.Errorf("offered type %q does not validate: %v", each, err)
		}
	}
}

func TestMessageAddsTheIssueAsATrailer(t *testing.T) {
	t.Parallel()

	subject := convention.Subject{Type: fixType, Scope: "config", Description: redactTokens, Breaking: false}

	cases := map[string]struct {
		body, key string
		want      string
	}{
		"subject, body and trailer": {
			body: "Tokens reached the log.\n", key: projKey,
			want: "fix(config): redact tokens\n\nTokens reached the log.\n\nRefs: PROJ-412\n",
		},
		"no body": {
			body: "", key: projKey,
			want: "fix(config): redact tokens\n\nRefs: PROJ-412\n",
		},
		"no issue": {
			body: "Why.", key: "",
			want: "fix(config): redact tokens\n\nWhy.\n",
		},
		"a trailer already written is not repeated": {
			body: "Why.\n\nRefs: PROJ-412", key: projKey,
			want: "fix(config): redact tokens\n\nWhy.\n\nRefs: PROJ-412\n",
		},
		// A key is not "present" merely because a longer key contains its text.
		"a longer key is not this key": {
			body: "Why.\n\nRefs: PROJ-4120", key: projKey,
			want: "fix(config): redact tokens\n\nWhy.\n\nRefs: PROJ-4120\nRefs: PROJ-412\n",
		},
		// The Refs trailer joins the trailer block rather than starting a new
		// paragraph, or git interpret-trailers loses the co-author.
		"the trailer joins an existing block": {
			body: "Why.\n\nCo-authored-by: Dev <dev@example.com>", key: projKey,
			want: "fix(config): redact tokens\n\nWhy.\n\nCo-authored-by: Dev <dev@example.com>\nRefs: PROJ-412\n",
		},
		// Only an empty line ends a paragraph, so a line of spaces inside the
		// block is passed over rather than taken for a line that is no trailer.
		"a line of spaces inside the block is passed over": {
			body: "Why.\n\nCo-authored-by: Dev <dev@example.com>\n  \nReviewed-by: Rev <rev@example.com>", key: projKey,
			want: "fix(config): redact tokens\n\nWhy.\n\nCo-authored-by: Dev <dev@example.com>\n  \n" +
				"Reviewed-by: Rev <rev@example.com>\nRefs: PROJ-412\n",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := convention.DefaultCommitConvention().Message(subject, tt.body, tt.key); got != tt.want {
				t.Errorf("Message() = %q, want %q", got, tt.want)
			}
		})
	}
}
