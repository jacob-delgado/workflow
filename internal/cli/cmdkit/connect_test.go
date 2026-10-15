// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestConnectFindsNoRepositoryWhereThereIsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	env, _ := inATempDir(t)

	var conn cmdkit.Connection

	root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
		var err error

		conn, err = cmdkit.Connect(cmd)
		conn.CloseLog()

		return err
	})

	// Act
	_, err := runTree(t, root, probeCommand)

	// Assert
	if err != nil || conn.Where.Repository {
		t.Errorf("connect = %v, wired to %+v; want the directory wired, as no repository", err, conn.Where)
	}

	if !errors.Is(conn.LoadErr, config.ErrNotFound) || conn.UnreadConfiguration() != nil {
		t.Errorf("load = %v, unread %v; want no file found, which is no refusal", conn.LoadErr, conn.UnreadConfiguration())
	}
}

func TestConnectRefusesAConfigurationItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	env, dir := inATempDir(t)
	clitest.WriteFile(t, dir, `{"jira": `)

	root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
		_, err := cmdkit.Connect(cmd)

		return err
	})

	// Act
	_, err := runTree(t, root, probeCommand)

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("connect over a file that cannot be read = %v, want it refused as config.ErrInvalid", err)
	}
}

func TestConnectLenientlyKeepsAConfigurationItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	env, dir := inATempDir(t)
	clitest.WriteFile(t, dir, `{"jira": `)

	var conn cmdkit.Connection

	root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
		var err error

		conn, err = cmdkit.ConnectLeniently(cmd)

		return err
	})

	// Act
	_, err := runTree(t, root, probeCommand)

	// Assert
	if err != nil || !errors.Is(conn.UnreadConfiguration(), config.ErrInvalid) {
		t.Errorf("connect leniently = %v, unread %v; want it wired, keeping why the file was not read",
			err, conn.UnreadConfiguration())
	}
}

func TestConnectSaysWhyItCannotBeWired(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		env  func(t *testing.T) cmdkit.Environment
		args func(t *testing.T) []string
		want string
	}{
		"a working directory that cannot be named": {
			env:  fromARemovedDirectory,
			args: func(*testing.T) []string { return []string{probeCommand} },
			want: clitest.WorkdirUnread,
		},
		"a request log that cannot be opened": {
			env: func(t *testing.T) cmdkit.Environment {
				t.Helper()

				env, _ := inATempDir(t)

				return env
			},
			args: func(t *testing.T) []string {
				t.Helper()

				return []string{"--request-log", filepath.Join(t.TempDir(), "missing", "requests.log"), probeCommand}
			},
			want: "opening the request log",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := probeTree(tt.env(t), func(cmd *cobra.Command, _ []string) error {
				_, err := cmdkit.Connect(cmd)

				return err
			})

			// Act
			_, err := runTree(t, root, tt.args(t)...)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("connect with %s = %v, want it to say %q", name, err, tt.want)
			}
		})
	}
}

func TestOnlyAConnectionNotDryRunKeepsWhatItIsToldInTheStore(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		kept bool
	}{
		"a connection":                 {args: []string{probeCommand}, kept: true},
		"a connection under --dry-run": {args: []string{"--dry-run", probeCommand}, kept: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env, dir := inATempDir(t)

			var (
				scope string
				kept  bool
			)

			root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
				conn := cmdkit.ConnectAt(cmd.Context(), cmd, dir, nil)
				_ = conn.Deps.Store.RecordScope(cmd.Context(), "cli")
				scope, kept = conn.Deps.Store.LastScope(cmd.Context())

				return nil
			})

			// Act
			_, err := runTree(t, root, tt.args...)

			// Assert
			if err != nil || kept != tt.kept || (kept && scope != "cli") {
				t.Errorf("%s = %v, read back %q (%v); want it kept: %v", name, err, scope, kept, tt.kept)
			}
		})
	}
}

func TestASetUpConnectionKeepsATokenInTheKeychainUnlessDryRun(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args  []string
		keeps bool
	}{
		"a connection":                 {args: []string{probeCommand}, keeps: true},
		"a connection under --dry-run": {args: []string{"--dry-run", probeCommand}, keeps: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env, dir := inATempDir(t)
			keychain := func(context.Context, string, string) error { return nil }

			var conn cmdkit.Connection

			root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
				conn = cmdkit.ConnectAt(cmd.Context(), cmd, dir, nil).WithSetup(cmd, keychain)

				return nil
			})

			// Act
			_, err := runTree(t, root, tt.args...)

			// Assert
			if keeps := conn.Keychain != nil; err != nil || keeps != tt.keeps || conn.Deps.Settings.Setup.Offer == nil {
				t.Errorf("%s = %v, keeps a token %v; want the first run offered, keeping one: %v",
					name, err, keeps, tt.keeps)
			}
		})
	}
}

func TestForgeNounNamesTheForgeOrSaysTheForge(t *testing.T) {
	t.Parallel()

	cases := map[forge.Kind]string{
		forge.KindUnknown: "the forge",
		forge.KindGitHub:  "GitHub",
		forge.KindGitLab:  "GitLab",
	}

	for kind, want := range cases {
		t.Run(want, func(t *testing.T) {
			t.Parallel()

			// Act
			got := cmdkit.ForgeNoun(kind)

			// Assert
			if got != want {
				t.Errorf("ForgeNoun(%d) = %q, want %q", kind, got, want)
			}
		})
	}
}

func TestRequestTimeoutIsTheDefaultUntilOneIsSet(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		set  string
		want time.Duration
	}{
		"none set": {set: "", want: config.DefaultRequestTimeout},
		"one set":  {set: "3s", want: 3 * time.Second},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cfg := config.Config{Timing: config.Timing{RequestTimeout: tt.set}}

			// Act
			got := cmdkit.RequestTimeout(cfg)

			// Assert
			if got != tt.want || cmdkit.OnlineDoer(cfg) == nil {
				t.Errorf("RequestTimeout with %q set = %v, want %v and a transport bounded by it", tt.set, got, tt.want)
			}
		})
	}
}

func TestCompletionOffersNothingItCannotAsk(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		env  func(t *testing.T) cmdkit.Environment
		args []string
	}{
		"past the issue's key": {
			env: func(t *testing.T) cmdkit.Environment {
				t.Helper()

				env, _ := inATempDir(t)

				return env
			},
			args: []string{"PROJ-7"},
		},
		"from a working directory that cannot be named": {env: fromARemovedDirectory},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// A tracker that would offer a key, configured in the home
			// directory, whose file is read from wherever the command runs.
			reached := new(atomic.Bool)
			server := clitest.JiraServer(t, http.StatusOK, clitest.SearchFixture("PROJ-8"), reached)
			env := tt.env(t)
			clitest.WriteConfigFor(t, env.Process.Home, server.URL)

			var (
				offered   []string
				directive cobra.ShellCompDirective
			)

			root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
				offered, directive = cmdkit.CompleteAssignedIssues(cmd, tt.args, "")

				return nil
			})

			// Act
			_, err := runTree(t, root, probeCommand)

			// Assert
			if err != nil || offered != nil || directive != cobra.ShellCompDirectiveNoFileComp || reached.Load() {
				t.Errorf("completion %s = %v, offered %q (%d), asked the tracker %v; want nothing asked or offered, "+
					"and no files", name, err, offered, directive, reached.Load())
			}
		})
	}
}
