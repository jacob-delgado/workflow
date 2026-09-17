// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"io/fs"
	"slices"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// configNames are the files lefthook reads its configuration from.
func configNames() []string {
	return []string{
		"lefthook.yml", ".lefthook.yml", "lefthook.yaml", ".lefthook.yaml",
		"lefthook.toml", ".lefthook.toml", "lefthook.json", ".lefthook.json",
		".config/lefthook.yml", ".config/lefthook.yaml", ".config/lefthook.toml", ".config/lefthook.json",
	}
}

// HasConfig reports whether a repository already configures lefthook. A local
// override alone does not count: it amends a configuration rather than being
// one.
func HasConfig(repo fs.FS) bool {
	return slices.ContainsFunc(configNames(), func(name string) bool {
		_, err := fs.Stat(repo, name)

		return err == nil
	})
}

// RunCommand runs one hook through lefthook, as git would, without committing.
func RunCommand(dir, hook string) proc.Command {
	return proc.Command{Dir: dir, Name: "lefthook", Args: []string{"run", hook, "--no-tty"}, Env: nil}
}
