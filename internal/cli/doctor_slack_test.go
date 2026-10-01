// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

// `workflow doctor --online` asks Slack, here a fake on this machine
// (fakeSlack), whose a Slack user token is.

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestDoctorOnlineSaysWhoseAnAcceptedSlackUserTokenIs(t *testing.T) {
	// Arrange
	fakeSlack(t, nil)

	dir := t.TempDir()
	writeFile(t, dir, slackLoggedInConfig())

	// Act
	output, _ := run(t, dir, "doctor", "--online")

	// Assert
	row := credentialRow(output, "slack")
	if !strings.HasPrefix(row, "ana in Acme (user token, kept in the configuration file ") ||
		!strings.HasSuffix(row, config.FileName+", expires in 1h0m0s)") {
		t.Errorf("doctor's slack row = %q, want ana in Acme, where the token is kept and when it expires:\n%s",
			row, output)
	}
}

// credentialRow is what doctor's credential check prints for service.
func credentialRow(output, service string) string {
	for line := range strings.SplitSeq(output, "\n") {
		if row, found := strings.CutPrefix(strings.TrimSpace(line), service+" "); found {
			return strings.TrimSpace(row)
		}
	}

	return ""
}
