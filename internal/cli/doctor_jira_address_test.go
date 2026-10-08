// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

// The login a jira.base_url carries, each part distinct enough that finding it
// in doctor's output can only mean doctor printed it.
const (
	addressUser     = "address-user-4417"
	addressPassword = "address-password-9253"
)

// addressWithLogin is a jira.base_url carrying that login.
const addressWithLogin = "https://" + addressUser + ":" + addressPassword + "@jira.example.com"

func TestDoctorJSONOnlineAsksNoJiraAtAnAddressItCannotUse(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"not an http or https address": "ftp://jira.example.com",
		"an address carrying a login":  addressWithLogin,
	}

	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The file does not load, so the run exits 3 for the address it
			// names; Jira is never asked, so no credential is what is wrong.
			dir := t.TempDir()
			writeConfigFor(t, dir, address)

			// Act
			output, err := run(t, dir, "doctor", "--json", "--online")

			// Assert
			wantExit(t, err, 3)

			report := decodeReport(t, output)
			if problem, _ := report["config_problem"].(string); !strings.Contains(problem, "jira.base_url") {
				t.Errorf("config_problem = %q, want it to name jira.base_url:\n%s", problem, output)
			}

			if credentials, _ := report["credentials"].(map[string]any); credentials["checked"] != false {
				t.Errorf("the online report checked a credential at an unusable address:\n%s", output)
			}
		})
	}
}

func TestDoctorRefusesAForgeKindNamingNoForge(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := forgeKindRepository(t, `"kind": "bitbucket", "host": "`+unreachableHost+`"`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	wantExit(t, err, 3)

	if !strings.Contains(output, `forge.kind is not github or gitlab: "bitbucket"`) {
		t.Errorf("doctor did not refuse forge.kind by name:\n%s", output)
	}
}

func TestDoctorOnlinePrintsNoPartOfTheLoginInJiraAddress(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"the prose report": "doctor --online",
		"the JSON report":  "doctor --json --online",
	}

	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			writeConfigFor(t, dir, addressWithLogin)

			// Act
			output, err := run(t, dir, strings.Fields(command)...)

			// Assert
			wantExit(t, err, 3)

			for _, part := range []string{addressUser, addressPassword} {
				if strings.Contains(output, part) || (err != nil && strings.Contains(err.Error(), part)) {
					t.Errorf("%s printed %q from jira.base_url's login: %v\n%s", command, part, err, output)
				}
			}
		})
	}
}
