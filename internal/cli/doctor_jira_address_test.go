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

func TestDoctorJSONOnlineCallsJiraUncheckedAtAnAddressItCannotUse(t *testing.T) {
	cases := map[string]string{
		"not an http or https address": "ftp://jira.example.com",
		"an address carrying a login":  addressWithLogin,
	}

	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			// The configuration section fails the address, so the run still exits 3;
			// Jira is never asked, so no credential is what is wrong.
			dir := t.TempDir()
			writeConfigFor(t, dir, address)

			// Act
			output, err := run(t, dir, "doctor", "--json", "--online")

			// Assert
			wantExit(t, err, 3)

			if got := credentialStatusIn(decodeReport(t, output), jiraService); got != uncheckedStatus {
				t.Errorf("the online report calls a Jira at an unusable address %q, want unchecked:\n%s", got, output)
			}

			if err != nil && strings.Contains(err.Error(), "a credential was rejected") {
				t.Errorf("doctor --json --online = %v, want no credential rejected when Jira was never asked", err)
			}
		})
	}
}

func TestDoctorOnlinePrintsNoPartOfTheLoginInJiraAddress(t *testing.T) {
	cases := map[string]string{
		"the prose report": "doctor --online",
		"the JSON report":  "doctor --json --online",
	}

	for name, command := range cases {
		t.Run(name, func(t *testing.T) {
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
