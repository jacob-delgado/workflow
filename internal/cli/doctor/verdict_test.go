// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"os"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

// The verdict each kind of fault earns, compared whole, in the order a
// script reads the lines of a joined error.

func verdictCases() map[string]fixedCase {
	return map[string]fixedCase{
		"a field missing": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, missingAField(t))
			},
			read: ran.verdict, want: "configuration is incomplete: 1 field(s) missing",
		},
		"values set wrong": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, settingAValueWrong(t))
			},
			read: ran.verdict, want: "configuration has an invalid value: 2 invalid value(s)",
		},
		"a file others can reach": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, sharingTheFile(t))
			},
			read: ran.verdict, want: "the configuration file can be reached by other users: mode 0644",
		},
		"a credential refused": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, refusingTheToken(t), "--online")
			},
			read: ran.verdict, want: "a credential was rejected: jira",
		},
		"no token at all": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, leavingOutTheToken(t), "--online")
			},
			read: ran.verdict, want: "configuration is incomplete: 1 field(s) missing\na credential is missing: jira",
		},
		"a token variable that gives nothing": {
			run:  onlineRowCases()["a token variable that gives nothing"].run,
			read: ran.verdict, want: "a credential is missing: jira",
		},
		"a service never reached": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, pointingAtNoHost(t), "--online")
			},
			read: ran.verdict, want: "the service could not be reached: jira",
		},
		"git missing": {run: withoutGit, read: ran.verdict, want: "required tooling is missing: git"},
		"git missing beside a file that does not parse": {
			run: func(t *testing.T) ran {
				t.Helper()

				clitest.SetVariable(t, "PATH", t.TempDir())
				dir := t.TempDir()
				clitest.WriteFile(t, dir, `{"jira": `)

				return doctorIn(t, dir)
			},
			// The file's path is the test's own; the verdict is held to the rest.
			read: func(done ran) string { return strings.ReplaceAll(done.verdict(), done.dir, "DIR") },
			want: "required tooling is missing: git\nDIR/" + config.FileName + ": invalid " + config.FileName +
				": unexpected EOF",
		},
		"every kind of fault at once": {
			run: func(t *testing.T) ran {
				t.Helper()

				clitest.SetVariable(t, "PATH", t.TempDir())
				dir := t.TempDir()

				err := os.Chmod(clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", `+
					`"token": "t"}, "ui": {"keys": {"commit": "a"}}}`), 0o644)
				if err != nil {
					t.Fatalf("chmod: %v", err)
				}

				return doctorIn(t, dir)
			},
			read: ran.verdict,
			want: "required tooling is missing: git\n" +
				"the configuration file can be reached by other users: mode 0644\n" +
				"configuration is incomplete: 1 field(s) missing\n" +
				"configuration has an invalid value: 1 invalid value(s)",
		},
	}
}

func TestDoctorGivesEachVerdictWordForWord(t *testing.T) {
	t.Parallel()

	for name, tt := range verdictCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.read(tt.run(t))

			// Assert
			if got != tt.want {
				t.Errorf("doctor = %q, want %q", got, tt.want)
			}
		})
	}
}
