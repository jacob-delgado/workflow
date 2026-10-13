// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"io/fs"
	"slices"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// configFiles are the files lefthook looks for under each of names, as its
// loader does: the name with every extension it parses.
func configFiles(names ...string) []string {
	extensions := []string{".yml", ".yaml", ".json", ".toml"}
	files := make([]string, 0, len(names)*len(extensions))

	for _, name := range names {
		for _, extension := range extensions {
			files = append(files, name+extension)
		}
	}

	return files
}

// mainConfigs are the files lefthook reads its configuration from.
func mainConfigs() []string {
	return configFiles("lefthook", ".lefthook", ".config/lefthook")
}

// HasConfig reports whether a repository already configures lefthook. A local
// override alone does not count: it amends a configuration rather than being
// one.
func HasConfig(repo fs.FS) bool {
	return slices.ContainsFunc(mainConfigs(), func(name string) bool {
		_, err := fs.Stat(repo, name)

		return err == nil
	})
}

// RunCommand runs one hook through lefthook, as git would, without committing.
func RunCommand(dir, hook string) proc.Command {
	return proc.Command{Dir: dir, Name: "lefthook", Args: []string{"run", hook, "--no-tty"}, Env: nil}
}

// InstallCommand installs lefthook's hooks in a repository: its root, not
// wherever the process happens to be.
func InstallCommand(dir string) proc.Command {
	return proc.Command{Dir: dir, Name: "lefthook", Args: []string{"install"}, Env: nil}
}
