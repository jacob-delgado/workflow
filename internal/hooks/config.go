// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// Runner runs a program and returns its standard output. proc.Run satisfies it.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

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

// wireHook is one hook in lefthook's dump: its jobs in any of the three forms
// lefthook accepts.
type wireHook struct {
	Commands map[string]json.RawMessage `json:"commands"`
	Scripts  map[string]json.RawMessage `json:"scripts"`
	Jobs     []struct {
		Name string `json:"name"`
	} `json:"jobs"`
}

// ReadConfig reads which hooks lefthook is configured to run, and the names of
// each one's jobs, from `lefthook dump --format json`. Asking lefthook, rather
// than parsing its YAML here, means local overrides, remotes and extends are
// already applied — and needs no YAML parser.
func ReadConfig(ctx context.Context, run Runner) (map[string][]string, error) {
	out, err := run(ctx, "lefthook", "dump", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("reading the lefthook configuration: %w", err)
	}

	var dump map[string]json.RawMessage

	err = json.Unmarshal(out, &dump)
	if err != nil {
		return nil, fmt.Errorf("reading the lefthook configuration: %w", err)
	}

	configured := map[string][]string{}

	for _, name := range gitHookNames() {
		raw, present := dump[name]
		if !present {
			continue
		}

		var hook wireHook

		// Settings outside a hook, such as colors, are not hooks; a hook that
		// does not decode is left out rather than failing the rest.
		if json.Unmarshal(raw, &hook) == nil {
			configured[name] = hook.names()
		}
	}

	return configured, nil
}

// names lists a hook's jobs by name, in the order lefthook runs piped ones.
func (w wireHook) names() []string {
	names := make([]string, 0, len(w.Commands)+len(w.Scripts)+len(w.Jobs))

	for name := range w.Commands {
		names = append(names, name)
	}

	for name := range w.Scripts {
		names = append(names, name)
	}

	for _, job := range w.Jobs {
		names = append(names, job.Name)
	}

	slices.Sort(names)

	return names
}

// RunCommand runs one hook through lefthook, as git would, without committing.
func RunCommand(dir, hook string) proc.Command {
	return proc.Command{Dir: dir, Name: "lefthook", Args: []string{"run", hook, "--no-tty"}, Env: nil}
}
