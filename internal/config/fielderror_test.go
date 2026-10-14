// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// refusedKeys are the keys of the settings err refused, in order.
func refusedKeys(err error) []string {
	fields := config.FieldErrors(err)

	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}

	return keys
}

func TestEachValidationNamesTheSettingItRefuses(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		contents, key string
	}{
		"a version this build does not read": {`{"version":"99"}`, "version"},
		"a Jira address that is not https":   {`{"jira":{"base_url":"ftp://jira.example.com"}}`, "jira.base_url"},
		"a forge that is neither":            {`{"forge":{"kind":"bitbucket"}}`, "forge.kind"},
		"a request timeout that is no time":  {`{"timing":{"request_timeout":"soon"}}`, "timing.request_timeout"},
		"a CI interval below zero":           {`{"timing":{"ci_interval":"-1s"}}`, "timing.ci_interval"},
		"a branch template with no key":      {`{"branch":{"template":"feature/{summary}"}}`, "branch.template"},
		"a negative slug limit":              {`{"branch":{"slug_limit":-1}}`, "branch.slug_limit"},
		"two prefixes for one issue type":    {`{"branch":{"prefixes":{"Bug":"fix","bug":"fix"}}}`, "branch.prefixes"},
		"a view with no name":                {`{"jira":{"views":[{"name":"","jql":"project = API"}]}}`, "jira.views"},
		"two headers that are one":           {`{"jira":{"headers":{"X-Team":"a","x-team":"b"}}}`, "jira.headers"},
		"a default scope with a space":       {`{"commit":{"default_scope":"two words"}}`, "commit.default_scope"},
		"a commit type that is no word":      {`{"commit":{"types":["not a type"]}}`, "commit.types"},
		"a negative subject limit":           {`{"commit":{"subject_limit":-1}}`, "commit.subject_limit"},
		"a refs trailer with its colon":      {`{"commit":{"refs_trailer":"Refs:"}}`, "commit.refs_trailer"},
		"a title source that is neither":     {`{"pull_request":{"title_source":"branch"}}`, "pull_request.title_source"},
		"a messaging kind this build lacks":  {`{"messaging":{"kind":"pigeon"}}`, "messaging.kind"},
		"a user token on a webhook's kind":   {`{"messaging":{"kind":"teams","client_id":"1.2"}}`, "messaging.kind"},
		"both of Slack's transports": {
			`{"messaging":{"kind":"slack","client_id":"1.2","webhook_url":"https://hooks.slack.com/a"}}`,
			"messaging.webhook_url",
		},
		"an expiry that is no time":       {`{"messaging":{"expires_at":"tomorrow"}}`, "messaging.expires_at"},
		"a negative comment count":        {`{"ui":{"comments_shown":-1}}`, "ui.comments_shown"},
		"a color other than never":        {`{"ui":{"color":"pink"}}`, "ui.color"},
		"a motion other than reduce":      {`{"ui":{"motion":"still"}}`, "ui.motion"},
		"a task program by relative path": {`{"taskwarrior":{"program":"bin/task"}}`, "taskwarrior.program"},
	}

	for name, refused := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := config.Parse(strings.NewReader(refused.contents))

			// Assert
			if keys := refusedKeys(err); !slices.Equal(keys, []string{refused.key}) {
				t.Errorf("refused %q (%v), want %s", keys, err, refused.key)
			}
		})
	}
}

func TestEverySettingRefusedIsNamedWithItsOwnReason(t *testing.T) {
	t.Parallel()

	// Arrange
	contents := `{"ui":{"color":"pink"},"timing":{"ci_interval":"soon"}}`

	// Act
	_, err := config.Parse(strings.NewReader(contents))

	// Assert
	fields := config.FieldErrors(err)
	if len(fields) != 2 || fields[0].Key != "timing.ci_interval" || fields[1].Key != "ui.color" ||
		!strings.Contains(fields[0].Error(), `"soon"`) || !strings.Contains(fields[1].Error(), `"pink"`) {
		t.Errorf("fields %v, want the CI interval then the color, each with its own value", fields)
	}
}

func TestARefusedSettingStillReadsAsItsKindOfRefusal(t *testing.T) {
	t.Parallel()

	// Arrange
	_, err := config.Parse(strings.NewReader(`{"branch":{"slug_limit":-1}}`))

	// Act
	fields := config.FieldErrors(fmt.Errorf("reading the repository's file: %w", err))

	// Assert
	if len(fields) != 1 || !errors.Is(fields[0], config.ErrInvalidBranch) || !errors.Is(err, config.ErrInvalid) {
		t.Errorf("fields %v of %v, want the slug limit, an invalid branch template", fields, err)
	}
}

func TestAKeyTheFileDoesNotHaveNamesNoSetting(t *testing.T) {
	t.Parallel()

	// Arrange
	_, err := config.Parse(strings.NewReader(`{"branches":{"slug_limit":-1}}`))

	// Act
	fields := config.FieldErrors(err)

	// Assert
	if err == nil || len(fields) != 0 {
		t.Errorf("fields %v of %v, want none named for a key the decoder refused", fields, err)
	}
}

func TestARefusedAddressNeverQuotesItsPassword(t *testing.T) {
	t.Parallel()

	// Arrange
	_, err := config.Parse(strings.NewReader(`{"jira":{"base_url":"https://ana:hunter2@jira.example.com"}}`))

	// Act
	fields := config.FieldErrors(err)

	// Assert
	if len(fields) != 1 || fields[0].Key != "jira.base_url" || strings.Contains(fields[0].Error(), "hunter2") {
		t.Errorf("fields %v, want jira.base_url refused without its password", fields)
	}
}
