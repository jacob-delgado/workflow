// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/proc"
)

func TestACommandRunWithoutAnEnvironmentHasNoneToRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// A command no root handed an environment over to, as one under a
	// persistent hook of its own would be.
	cmd := &cobra.Command{}
	cmd.SetContext(t.Context())

	// Act
	env := cmdkit.EnvironmentOf(cmd)

	// Assert
	_, dirErr := cmdkit.WorkingDir(cmd)
	_, stateErr := env.Process.StateDir()
	_, lookErr := env.Process.LookPath("git")

	if dirErr == nil || stateErr == nil || env.Chdir(t.TempDir()) == nil || !errors.Is(lookErr, proc.ErrNotFound) {
		t.Errorf("working directory %v, store %v, programs %v; want each refused", dirErr, stateErr, lookErr)
	}

	if env.Process.Home != "" || env.Process.Getenv("HOME") != "" || cmdkit.ConfigHome(cmd) != "" {
		t.Errorf("home %q, $HOME %q; want neither named", env.Process.Home, env.Process.Getenv("HOME"))
	}
}

func TestACommandReadsItsWorkingDirectoryOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	env, dir := inATempDir(t)
	reads := 0
	env.WorkingDir = func() (string, error) {
		reads++

		return dir, nil
	}

	var first, second string

	root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
		first, _ = cmdkit.WorkingDir(cmd)
		second, _ = cmdkit.WorkingDir(cmd)

		return nil
	})

	// Act
	_, err := runTree(t, root, probeCommand)

	// Assert
	if err != nil || reads != 1 || first != dir || second != dir {
		t.Errorf("run = %v, read %d times as %q then %q; want %q, read once", err, reads, first, second, dir)
	}
}

func TestFromWorkingDirReadsAPathAsAShellThereWould(t *testing.T) {
	t.Parallel()

	absolute := filepath.Join(t.TempDir(), "requests.log")

	cases := map[string]struct {
		path string
		want func(dir string) string
	}{
		"a relative path": {
			path: "logs/requests.log", want: func(dir string) string { return filepath.Join(dir, "logs/requests.log") },
		},
		"an absolute path": {path: absolute, want: func(string) string { return absolute }},
		"no path":          {path: "", want: func(dir string) string { return dir }},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			env, dir := inATempDir(t)

			var found string

			root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
				var err error

				found, err = cmdkit.FromWorkingDir(cmd, tt.path)

				return err
			})

			// Act
			_, err := runTree(t, root, probeCommand)

			// Assert
			if want := tt.want(dir); err != nil || found != want {
				t.Errorf("%s = %q, %v; want %q", name, found, err, want)
			}
		})
	}
}

func TestFromWorkingDirNeedsTheWorkingDirectoryOnlyForARelativePath(t *testing.T) {
	t.Parallel()

	absolute := filepath.Join(t.TempDir(), "requests.log")

	cases := map[string]struct {
		path    string
		refused bool
	}{
		"a relative path":  {path: "requests.log", refused: true},
		"an absolute path": {path: absolute, refused: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			root := probeTree(fromARemovedDirectory(t), func(cmd *cobra.Command, _ []string) error {
				_, err := cmdkit.FromWorkingDir(cmd, tt.path)

				return err
			})

			// Act
			_, err := runTree(t, root, probeCommand)

			// Assert
			if refused := err != nil && strings.Contains(err.Error(), workdirUnread); refused != tt.refused {
				t.Errorf("%s from a removed directory = %v, want it refused: %v", name, err, tt.refused)
			}
		})
	}
}

func TestLoadFromEnvironmentReadsTheFileWhereTheCommandRuns(t *testing.T) {
	t.Parallel()

	// Arrange
	env, dir := inATempDir(t)
	clitest.WriteFile(t, dir, `{"jira": {"base_url": "https://jira.example.com"}}`)

	var baseURL string

	root := probeTree(env, func(cmd *cobra.Command, _ []string) error {
		cfg, err := cmdkit.LoadFromEnvironment(cmd)
		baseURL = cfg.Jira.BaseURL

		return err
	})

	// Act
	_, err := runTree(t, root, probeCommand)

	// Assert
	if err != nil || baseURL != "https://jira.example.com" {
		t.Errorf("load = %v, Jira at %q; want the file in the working directory read", err, baseURL)
	}
}

func TestLoadFromEnvironmentSaysWhenTheWorkingDirectoryCannotBeNamed(t *testing.T) {
	t.Parallel()

	// Arrange
	root := probeTree(fromARemovedDirectory(t), func(cmd *cobra.Command, _ []string) error {
		_, err := cmdkit.LoadFromEnvironment(cmd)

		return err
	})

	// Act
	_, err := runTree(t, root, probeCommand)

	// Assert
	if err == nil || !strings.Contains(err.Error(), workdirUnread) {
		t.Errorf("load from a removed directory = %v, want it to say it cannot name the directory", err)
	}
}
