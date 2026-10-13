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
	extensions := []string{".yml", ".yaml", ".json", ".jsonc", ".toml"}
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

// localConfigs are the files lefthook reads a local override from: one that
// amends the configuration, or stands in for it where there is none.
func localConfigs() []string {
	return configFiles("lefthook-local", ".lefthook-local", ".config/lefthook-local")
}

// holdsAny reports whether a repository holds any of files.
func holdsAny(repo fs.FS, files []string) bool {
	return slices.ContainsFunc(files, func(name string) bool {
		_, err := fs.Stat(repo, name)

		return err == nil
	})
}

// HasConfig reports whether a repository already configures lefthook. A local
// override alone does not count: it amends a configuration rather than being
// one.
func HasConfig(repo fs.FS) bool {
	return holdsAny(repo, mainConfigs())
}

// lefthook is the program every command line here runs.
const lefthook = "lefthook"

// RunCommand runs one hook through lefthook, as git would, without committing.
func RunCommand(dir, hook string) proc.Command {
	return proc.Command{Dir: dir, Name: lefthook, Args: []string{"run", hook, "--no-tty"}, Env: nil}
}

// InstallCommand installs lefthook's hooks in a repository: its root, not
// wherever the process happens to be.
func InstallCommand(dir string) proc.Command {
	return proc.Command{Dir: dir, Name: lefthook, Args: []string{"install"}, Env: nil}
}

// DumpCommand prints lefthook's configuration as JSON, merged as a run of it
// reads it: the repository's file, its local override, and the remotes it
// names, from lefthook's own cache.
//
// lefthook logs on the standard output it dumps to, so a LEFTHOOK_VERBOSE
// this process inherited would put its debug lines ahead of the JSON. The
// command turns it off; a duplicated variable takes its last value.
func DumpCommand(dir string) proc.Command {
	return proc.Command{
		Dir: dir, Name: lefthook, Args: []string{"dump", "--format", "json"}, Env: []string{"LEFTHOOK_VERBOSE=0"},
	}
}
