// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"strings"
	"testing"
)

func TestDoctorOfflineSaysWhatOnlineAsksAndThatAWebhookStaysUnchecked(t *testing.T) {
	t.Parallel()

	// Arrange
	// A webhook is the one messaging credential --online cannot ask about
	// without posting, so the offline line must not promise it.
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`)

	// Act
	output, err := run(t, dir, "doctor")

	// Assert
	const want = "Credentials were not checked. Add --online to ask Jira, your forge and Slack; " +
		"a webhook is left unchecked."
	if err != nil || !strings.Contains(output, want) {
		t.Errorf("doctor = %v, want success saying %q:\n%s", err, want, output)
	}
}
