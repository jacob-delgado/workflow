// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

// credentialedHome is a home file holding a credential for every section that
// sends one somewhere: Jira's token and headers, the forge's token and a Slack
// webhook.
const credentialedHome = `{
  "jira": {"base_url": "https://jira.example.com", "token": "jira-token-home-1234",
    "headers": {"X-Auth": "header-home-1234"}},
  "forge": {"host": "git.example.com", "token": "forge-token-home-1234"},
  "messaging": {"webhook_url": "https://hooks.slack.example/home"}
}`

// tokenSourcesHome is a home file naming where Jira's token comes from rather
// than holding it.
const tokenSourcesHome = `{"jira": {"base_url": "https://jira.example.com",
  "token_command": "pass show jira", "token_env": "JIRA_TOKEN"}}`

// slackUserHome is a home file posting with a Slack user token.
const slackUserHome = `{"messaging": {"client_id": "1.2", "client_secret": "client-secret-home",
  "refresh_token": "xoxe-1-home", "access_token": "xoxe.xoxp-home", "expires_at": "2026-01-01T00:00:00Z"}}`

// heldCredentials is every credential a configuration carries, revealed so a
// test can say which file each came from.
type heldCredentials struct {
	JiraToken, JiraCommand, JiraEnv string
	JiraHeaders                     map[string]string
	ForgeToken                      string
	WebhookURL                      string
	SlackSecrets                    string
}

// credentialsOf reveals every credential cfg holds.
func credentialsOf(cfg config.Config) heldCredentials {
	var headers map[string]string

	for name, value := range cfg.Jira.Headers {
		if headers == nil {
			headers = map[string]string{}
		}

		headers[name] = value.Reveal()
	}

	return heldCredentials{
		JiraToken: cfg.Jira.Token.Reveal(), JiraCommand: cfg.Jira.TokenCommand, JiraEnv: cfg.Jira.TokenEnv,
		JiraHeaders: headers, ForgeToken: cfg.Forge.Token.Reveal(), WebhookURL: cfg.Messaging.WebhookURL.Reveal(),
		SlackSecrets: cfg.Messaging.ClientSecret.Reveal() + cfg.Messaging.RefreshToken.Reveal() +
			cfg.Messaging.AccessToken.Reveal(),
	}
}

// homeCredentials is what credentialedHome holds.
func homeCredentials() heldCredentials {
	return heldCredentials{
		JiraToken: "jira-token-home-1234", JiraHeaders: map[string]string{"X-Auth": "header-home-1234"},
		ForgeToken: "forge-token-home-1234", WebhookURL: "https://hooks.slack.example/home",
	}
}

// homeCredentialsExcept is homeCredentials with change applied.
func homeCredentialsExcept(change func(*heldCredentials)) heldCredentials {
	held := homeCredentials()
	change(&held)

	return held
}

// layeredOver writes home and repo to files of their own.
func layeredOver(t *testing.T, home, repo string) config.Files {
	t.Helper()

	return config.Files{Home: write(t, t.TempDir(), home), Repo: write(t, t.TempDir(), repo)}
}

func TestARepositoryLayerBorrowsAHomeCredentialOnlyForTheSameAddress(t *testing.T) {
	t.Parallel()

	withoutJira := func(held *heldCredentials) { held.JiraToken, held.JiraHeaders = "", nil }

	cases := []struct {
		name, home, repo string
		want             heldCredentials
	}{
		{
			name: "jira moved elsewhere", home: credentialedHome,
			repo: `{"jira": {"base_url": "https://evil.example"}}`, want: homeCredentialsExcept(withoutJira),
		},
		{
			name: "jira at the same address", home: credentialedHome,
			repo: `{"jira": {"base_url": "https://jira.example.com", "project": "OSS"}}`, want: homeCredentials(),
		},
		{
			name: "jira address left alone", home: credentialedHome,
			repo: `{"jira": {"project": "OSS"}}`, want: homeCredentials(),
		},
		{
			name: "jira moved with a token of its own", home: credentialedHome,
			repo: `{"jira": {"base_url": "https://other.example", "token": "jira-token-repo-5678"}}`,
			want: homeCredentialsExcept(func(held *heldCredentials) {
				held.JiraToken, held.JiraHeaders = "jira-token-repo-5678", nil
			}),
		},
		{
			name: "jira moved from a token's sources", home: tokenSourcesHome,
			repo: `{"jira": {"base_url": "https://evil.example"}}`, want: heldCredentials{},
		},
		{
			name: "jira kept with a token's sources", home: tokenSourcesHome,
			repo: `{"jira": {"project": "OSS"}}`,
			want: heldCredentials{JiraCommand: "pass show jira", JiraEnv: "JIRA_TOKEN"},
		},
		{
			name: "forge moved elsewhere", home: credentialedHome, repo: `{"forge": {"host": "evil.example"}}`,
			want: homeCredentialsExcept(func(held *heldCredentials) { held.ForgeToken = "" }),
		},
		{
			name: "forge kind named on the same host", home: credentialedHome,
			repo: `{"forge": {"kind": "gitlab"}}`, want: homeCredentials(),
		},
		{
			name: "messaging moved to another service", home: credentialedHome,
			repo: `{"messaging": {"kind": "discord"}}`,
			want: homeCredentialsExcept(func(held *heldCredentials) { held.WebhookURL = "" }),
		},
		{
			name: "messaging naming the service it already used", home: credentialedHome,
			repo: `{"messaging": {"kind": "slack", "channel": "#oss"}}`, want: homeCredentials(),
		},
		{
			name: "slack user token not borrowed by teams", home: slackUserHome,
			repo: `{"messaging": {"kind": "teams", "webhook_url": "https://teams.example/repo"}}`,
			want: heldCredentials{WebhookURL: "https://teams.example/repo"},
		},
		{
			name: "slack user token kept for another channel", home: slackUserHome,
			repo: `{"messaging": {"channel": "#oss"}}`,
			want: heldCredentials{SlackSecrets: "client-secret-homexoxe-1-homexoxe.xoxp-home"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := layeredOver(t, testCase.home, testCase.repo)

			// Act
			cfg, _, err := config.LoadLayersAt(files)

			// Assert
			got := credentialsOf(cfg)
			if err != nil || !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("credentials = %+v, %v; want %+v", got, err, testCase.want)
			}
		})
	}
}

func TestMovingJiraInTheRepositoryNeverCopiesTheHomeCredentialsThere(t *testing.T) {
	t.Parallel()

	// Arrange
	files := layeredOver(t, credentialedHome, `{}`)

	cfg, over, err := config.LoadLayersAt(files)
	if err != nil {
		t.Fatalf("loading: %v", err)
	}

	cfg.Jira.BaseURL = "https://other.example"

	// Act
	_, err = config.SaveLayers(files, cfg, over)

	// Assert
	written, readErr := os.ReadFile(files.Repo)
	if err != nil || readErr != nil {
		t.Fatalf("saving = %v, reading back = %v", err, readErr)
	}

	for _, secret := range []string{"jira-token-home-1234", "header-home-1234", "forge-token-home-1234"} {
		if strings.Contains(string(written), secret) {
			t.Errorf("the repository file holds the home file's %q:\n%s", secret, written)
		}
	}

	reloaded, _, err := config.LoadLayersAt(files)
	if err != nil || reloaded.Jira.Token != "" || len(reloaded.Jira.Headers) != 0 {
		t.Errorf("reloaded jira = %+v, %v; want no home credential sent to the new address", reloaded.Redacted().Jira, err)
	}
}
