// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/doctor"
)

// A script reads doctor's verdict by its exit status, which the root's exit
// families find by these sentinels, so each finding is pinned to its own.
func TestDoctorsFindingsWrapTheSentinelsTheExitFamiliesName(t *testing.T) {
	t.Parallel()

	online := []string{"--online"}

	cases := map[string]struct {
		sentinel error
		args     []string
		setup    func(t *testing.T) string
	}{
		"a field missing":         {sentinel: doctor.ErrIncomplete, setup: missingAField},
		"a value set wrong":       {sentinel: doctor.ErrInvalid, setup: settingAValueWrong},
		"a file others can reach": {sentinel: doctor.ErrShared, setup: sharingTheFile},
		"a credential refused":    {sentinel: doctor.ErrCredentialRejected, args: online, setup: refusingTheToken},
		"no credential to ask":    {sentinel: doctor.ErrCredentialMissing, args: online, setup: leavingOutTheToken},
		"a service never reached": {sentinel: doctor.ErrUnreachable, args: online, setup: pointingAtNoHost},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := testCase.setup(t)

			// Act
			output, err := clitest.Run(t, dir, append([]string{doctorCommand}, testCase.args...)...)

			// Assert
			if !errors.Is(err, testCase.sentinel) {
				t.Errorf("doctor = %v, want it to wrap %q:\n%s", err, testCase.sentinel, output)
			}
		})
	}
}

// missingAField is a configuration with no messaging service.
func missingAField(t *testing.T) string {
	t.Helper()

	return configuredWith(t, `"jira": {"base_url": "https://jira.example.com", "token": "t"}`)
}

// settingAValueWrong is a configuration with every field set, some wrongly.
func settingAValueWrong(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	clitest.WriteFile(t, dir, invalidValuesConfig)

	return dir
}

// sharingTheFile is a complete configuration other users can read.
func sharingTheFile(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	err := os.Chmod(clitest.WriteFile(t, dir, completeConfig), 0o644)
	if err != nil {
		t.Fatalf("chmod: %v", err)
	}

	return dir
}

// refusingTheToken is a configuration whose Jira refuses its token.
func refusingTheToken(t *testing.T) string {
	t.Helper()

	server := clitest.JiraServer(t, http.StatusUnauthorized, "<html>login</html>", new(atomic.Bool))

	return configuredWith(t, `"jira": {"base_url": "`+server.URL+`", "token": "t"}, `+clitest.SlackWebhook)
}

// leavingOutTheToken is a configuration with no Jira token to ask with.
func leavingOutTheToken(t *testing.T) string {
	t.Helper()

	return configuredWith(t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`"}, `+clitest.SlackWebhook)
}

// pointingAtNoHost is a configuration whose Jira is at a host that never
// answers.
func pointingAtNoHost(t *testing.T) string {
	t.Helper()

	return configuredWith(t, `"jira": {"base_url": "https://jira.invalid", "token": "t"}, `+clitest.SlackWebhook)
}

func TestDoctorCountsWhatIsMissingAndWhatIsWrong(t *testing.T) {
	t.Parallel()

	// Arrange
	// commit is bound to a key stage-all holds, and there is no messaging
	// service.
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"}, `+
		`"ui": {"keys": {"commit": "a"}}}`)

	// Act
	_, err := clitest.Run(t, dir, "doctor")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "1 field(s) missing") ||
		!strings.Contains(err.Error(), "1 invalid value(s)") {
		t.Errorf("doctor = %v, want one field counted missing and one value counted invalid", err)
	}
}
