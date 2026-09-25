// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package convention_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/convention"
)

// Fixtures the tests share.
const (
	projKey    = "PROJ-412"
	taskType   = "Task"
	fixType    = "fix"
	featType   = "feat"
	choreType  = "chore"
	hotfixType = "hotfix"
	// redactTokens is a description that satisfies every rule, and
	// redactSubject the subject it makes.
	redactTokens  = "redact tokens"
	redactSubject = "fix(config): redact tokens"
	// issueSummary is the summary of the issue projKey names.
	issueSummary = "Fix token redaction"
)

func TestBranchNameReadsAsTheIssue(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		issueType, key, summary string
		want                    string
	}{
		"a bug is a fix": {
			issueType: "Bug", key: projKey, summary: issueSummary,
			want: "fix/PROJ-412-fix-token-redaction",
		},
		"anything else is a feature": {
			issueType: "Story", key: "PROJ-7", summary: "Add retries to the webhook client",
			want: "feat/PROJ-7-add-retries-to-the-webhook-client",
		},
		"the type is matched without regard to case": {
			issueType: "bug", key: "OPS-1", summary: "x", want: "fix/OPS-1-x",
		},
		"punctuation collapses to one hyphen": {
			issueType: taskType, key: "OPS-2", summary: "  Don't crash --- on: empty (nil) input!  ",
			want: "feat/OPS-2-don-t-crash-on-empty-nil-input",
		},
		"accents fold to ASCII": {
			issueType: taskType, key: "OPS-3", summary: "Café menü für Zoë",
			want: "feat/OPS-3-cafe-menu-fur-zoe",
		},
		"a summary with no letters leaves just the key": {
			issueType: taskType, key: "OPS-4", summary: "!!! ???", want: "feat/OPS-4",
		},
		"a long summary stops at a word": {
			issueType: taskType, key: "OPS-5",
			summary: "Replace the hand rolled retry loop in the webhook client with exponential backoff",
			want:    "feat/OPS-5-replace-the-hand-rolled-retry-loop-in-the",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := convention.DefaultBranchNaming().Name(tt.issueType, tt.key, tt.summary)

			// Assert
			if got != tt.want {
				t.Errorf("Name = %q, want %q", got, tt.want)
			}

			err := convention.ValidateBranchName(got)
			if err != nil {
				t.Errorf("Name proposed %q, which git would refuse: %v", got, err)
			}
		})
	}
}

func TestIssueKeyReadsAForgeNumberFromABranch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text string
		want string
	}{
		"a forge branch":           {text: "fix/42-fix-typo", want: "42"},
		"no prefix":                {text: "7-add-retries", want: "7"},
		"an empty slug":            {text: "feat/13", want: "13"},
		"a Jira key still wins":    {text: "fix/PROJ-1-thing", want: "PROJ-1"},
		"a slug number is no key":  {text: "fix/add-256-colors", want: ""},
		"a hash is no forge key":   {text: "chore/bump-sha-256", want: ""},
		"a leading zero is no key": {text: "fix/0-nope", want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.IssueKey(tt.text, "")

			// Assert
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("IssueKey(%q) = %q, %v, want %q", tt.text, got, ok, tt.want)
			}
		})
	}
}

func TestIssueKeyIsFoundWhereJiraWouldFindIt(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text string
		want string
	}{
		"a generated branch":      {text: "fix/PROJ-412-token-redaction", want: projKey},
		"a bare key":              {text: "OPS-7", want: "OPS-7"},
		"digits in the project":   {text: "feat/AB2C-10-thing", want: "AB2C-10"},
		"the first of two":        {text: "OPS-1-and-OPS-2", want: "OPS-1"},
		"a key inside a word":     {text: "xOPS-1", want: ""},
		"lowercase is not a key":  {text: "fix/utf-8-handling", want: ""},
		"a version is not a key":  {text: "release/V-0", want: ""},
		"one letter is not a key": {text: "A-1", want: ""},
		"nothing":                 {text: "main", want: ""},
		"a key followed by digit": {text: "OPS-12x", want: "OPS-12"},
		// Common standards read like a key but are not one.
		"an encoding is not a key": {text: "fix/UTF-8-decoding", want: ""},
		"a hash is not a key":      {text: "feat/SHA-256-support", want: ""},
		"a CVE is not a key":       {text: "fix/CVE-2024-1-patch", want: ""},
		// A real key after a standard is still found.
		"a standard then a key": {text: "fix/UTF-8-and-PROJ-412", want: projKey},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.IssueKey(tt.text, "")

			// Assert
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("IssueKey(%q) = %q, %v, want %q", tt.text, got, ok, tt.want)
			}
		})
	}
}

// "@" is refused, but not because it is empty — it is git's shorthand for the
// current branch — so the reason must not misreport it as blank.
func TestTheAtSignIsRefusedForWhatItIs(t *testing.T) {
	t.Parallel()

	// Act
	err := convention.ValidateBranchName("@")

	// Assert
	if err == nil || strings.Contains(err.Error(), "empty") {
		t.Errorf("ValidateBranchName(%q) = %v, want a reason that is not about emptiness", "@", err)
	}
}

func TestBranchTypeReadsThePrefixWhenItIsACommitType(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		branch string
		want   string
	}{
		"a fix branch":              {branch: "fix/PROJ-1-thing", want: fixType},
		"a feat branch":             {branch: "feat/OPS-2-add", want: featType},
		"a docs branch":             {branch: "docs/readme", want: "docs"},
		"a prefix that is no type":  {branch: "feature/OPS-3", want: ""},
		"no slash at all":           {branch: "main", want: ""},
		"a bare type is not a type": {branch: fixType, want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, ok := convention.DefaultCommitConvention().BranchType(tt.branch)

			// Assert
			if got != tt.want || ok != (tt.want != "") {
				t.Errorf("BranchType(%q) = %q, %v, want %q", tt.branch, got, ok, tt.want)
			}
		})
	}
}

func TestValidateBranchNameAcceptsWhatGitAccepts(t *testing.T) {
	t.Parallel()

	for _, branch := range []string{"feat/PROJ-1-x", "a", "fix/nested/deeper", "v1.2"} {
		t.Run(strconv.Quote(branch), func(t *testing.T) {
			t.Parallel()

			// Act
			err := convention.ValidateBranchName(branch)
			// Assert
			if err != nil {
				t.Errorf("ValidateBranchName(%q) = %v, want nil", branch, err)
			}
		})
	}
}

func TestValidateBranchNameRefusesWhatGitRefuses(t *testing.T) {
	t.Parallel()

	// Each is a rule of git check-ref-format --branch.
	for _, branch := range []string{
		"", "has space", "a..b", "a/.hidden", ".start", "end.", "end/", "/start", "a//b",
		"x.lock", "a/b.lock/c", "a@{b", "@", "-start", "tilde~", "caret^", "colon:",
		"question?", "star*", "bracket[", "back\\slash", "tab\tname", "del\x7f",
	} {
		t.Run(strconv.Quote(branch), func(t *testing.T) {
			t.Parallel()

			// Act
			err := convention.ValidateBranchName(branch)

			// Assert
			if !errors.Is(err, convention.ErrInvalidBranchName) {
				t.Errorf("ValidateBranchName(%q) = %v, want ErrInvalidBranchName", branch, err)
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
			subject: convention.Subject{Type: "feat", Scope: "cli", Description: "drop --old", Breaking: true},
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
		"no description": {
			change: func(s *convention.Subject) { s.Description = "  " }, want: convention.ErrNoDescription,
		},
		"a trailing period": {
			change: func(s *convention.Subject) { s.Description = "redact tokens." }, want: convention.ErrTrailingPeriod,
		},
		"too long": {
			change: func(s *convention.Subject) { s.Description = strings.Repeat("x", 68) },
			want:   convention.ErrSubjectTooLong,
		},
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

func TestIssueKeyRestrictsToTheConfiguredProject(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		text, project, want string
		found               bool
	}{
		"the configured project matches":     {text: "feat/PROJ-42-thing", project: "PROJ", want: "PROJ-42", found: true},
		"another project is not read as one": {text: "feat/ABC-123-thing", project: "PROJ", want: "", found: false},
		"no project falls back to the guard": {text: "fix/UTF-8-decoding", project: "", want: "", found: false},
		"no project still reads a real key":  {text: "feat/PROJ-42-thing", project: "", want: "PROJ-42", found: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, found := convention.IssueKey(tt.text, tt.project)

			// Assert
			if got != tt.want || found != tt.found {
				t.Errorf("IssueKey(%q, %q) = %q, %v; want %q, %v", tt.text, tt.project, got, found, tt.want, tt.found)
			}
		})
	}
}
