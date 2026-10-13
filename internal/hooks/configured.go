// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/jacob-delgado/workflow/internal/proc"
)

// ErrUnreadableConfiguration reports a dump of lefthook's configuration that
// is not one.
var ErrUnreadableConfiguration = errors.New("lefthook's configuration could not be read")

// Configured is the hooks lefthook's merged configuration defines, split by
// whether a run by hand can stand in for git's.
type Configured struct {
	// Runnable is the hooks a run by hand can stand in for: git's own in the
	// order git documents them, then the configuration's own names, sorted.
	Runnable []string
	// GitOnly is the hooks only git can run, in git's order.
	GitOnly []GitOnly
}

// GitOnly is a hook only git can run, and why: what git hands it that a run
// by hand would not.
type GitOnly struct {
	Name   string
	Reason string
}

// ReadConfigured reads the hooks lefthook's configuration in dir defines,
// asking lefthook through run for the configuration merged as a run of it
// reads it: whichever file lefthook finds, its local override even alone, or
// the file LEFTHOOK_CONFIG names. lefthook alone knows every place it looks,
// so it is always asked. It refuses to dump where it finds no configuration,
// and that refusal, from a directory holding no file lefthook reads, is no
// hook at all rather than a failure.
func ReadConfigured(
	ctx context.Context, dir string, run func(context.Context, proc.Command) ([]byte, error),
) (Configured, error) {
	dump, err := run(ctx, DumpCommand(dir))
	if err != nil && !holdsAny(os.DirFS(dir), slices.Concat(mainConfigs(), localConfigs())) {
		return Configured{}, nil
	}

	if err != nil {
		return Configured{}, fmt.Errorf("reading lefthook's configuration: %w", err)
	}

	names, err := hookNames(dump)
	if err != nil {
		return Configured{}, err
	}

	return split(ordered(names)), nil
}

// hookNames is every name in a dump that is a hook with something to run.
// lefthook dumps its settings beside its hooks, at the top level of one
// object, so a hook is told by its shape.
func hookNames(dump []byte) ([]string, error) {
	var entries map[string]json.RawMessage

	err := json.Unmarshal(dump, &entries)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreadableConfiguration, err)
	}

	maps.DeleteFunc(entries, func(_ string, value json.RawMessage) bool { return !definesJobs(value) })

	return slices.Collect(maps.Keys(entries)), nil
}

// hookShape is the part of a dumped hook that says it runs something: jobs,
// commands or scripts, each left out of the dump when there are none. No
// setting of lefthook's holds one.
type hookShape struct {
	Jobs     json.RawMessage `json:"jobs"`
	Commands json.RawMessage `json:"commands"`
	Scripts  json.RawMessage `json:"scripts"`
}

// definesJobs reports a dumped value that is a hook with something to run.
func definesJobs(value json.RawMessage) bool {
	var shape hookShape

	err := json.Unmarshal(value, &shape)
	if err != nil {
		return false
	}

	return shape.Jobs != nil || shape.Commands != nil || shape.Scripts != nil
}

// ordered is the names with git's hooks first, in the order git documents
// them, and then the configuration's own, sorted.
func ordered(names []string) []string {
	gits := gitHookNames()
	rank := func(name string) int {
		index := slices.Index(gits, name)
		if index < 0 {
			return len(gits)
		}

		return index
	}

	return slices.SortedFunc(slices.Values(names), func(first, second string) int {
		return cmp.Or(cmp.Compare(rank(first), rank(second)), cmp.Compare(first, second))
	})
}

// split sorts the names into the hooks a run by hand can stand in for and
// those only git can run, keeping their order.
func split(names []string) Configured {
	handed := handedByGit()

	var configured Configured

	for _, name := range names {
		what, gitOnly := handed[name]
		if !gitOnly {
			configured.Runnable = append(configured.Runnable, name)

			continue
		}

		configured.GitOnly = append(configured.GitOnly, GitOnly{Name: name, Reason: "git hands it " + what})
	}

	return configured
}
