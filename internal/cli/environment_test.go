// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/proc"
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// Every run a test makes is handed an Environment of its own, so the tests run
// side by side: none changes the process's working directory or environment.
// What a test sets of the outside world — a stand-in program found first on
// PATH, an environment variable — it keeps in a directory named for it, its
// world, which its runs read without being handed it, and which a subtest
// shares with the tests above it.

// worldsRoot is where every test's world lives while this package's tests run,
// named for the process so two runs of them never share one.
func worldsRoot() string {
	return filepath.Join(os.TempDir(), "workflow-cli-tests-"+strconv.Itoa(os.Getpid()))
}

// worldOf is the directory test t keeps what its runs see of the outside world
// in, made on first use and removed when the test ends.
func worldOf(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(worldsRoot(), worldPath(t.Name()))

	_, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
	}

	return dir
}

// programsOf is the directory test t's stand-in programs are in, found first
// on its runs' PATH.
func programsOf(t *testing.T) string {
	t.Helper()

	dir := filepath.Join(worldOf(t), "bin")

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		t.Fatalf("making the directory for stand-in programs: %v", err)
	}

	return dir
}

// setVariable sets an environment variable for test t's runs, and for its
// subtests'.
func setVariable(t *testing.T, name, value string) {
	t.Helper()

	dir := filepath.Join(worldOf(t), "env")

	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		t.Fatalf("making the directory for environment variables: %v", err)
	}

	err = os.WriteFile(filepath.Join(dir, name), []byte(value), 0o600)
	if err != nil {
		t.Fatalf("setting %s: %v", name, err)
	}
}

// worldsAbove is test t's world and those of the tests it runs under, its own
// first.
func worldsAbove(t *testing.T) []string {
	t.Helper()

	names := strings.Split(t.Name(), "/")
	worlds := make([]string, 0, len(names))

	for depth := len(names); depth > 0; depth-- {
		worlds = append(worlds, filepath.Join(worldsRoot(), worldPath(strings.Join(names[:depth], "/"))))
	}

	return worlds
}

// worldPath is where under worldsRoot the world of the test named name is: a
// directory per level of its name, each escaped, so a subtest named with a
// colon, the PATH list's separator, still makes a directory PATH can name.
func worldPath(name string) string {
	levels := strings.Split(name, "/")
	for at, level := range levels {
		levels[at] = url.QueryEscape(level)
	}

	return filepath.Join(levels...)
}

// variableSet is the value a test above or at t set for name, and whether one
// did.
func variableSet(t *testing.T, name string) (string, bool) {
	t.Helper()

	for _, world := range worldsAbove(t) {
		value, err := os.ReadFile(filepath.Join(world, "env", name))
		if err == nil {
			return string(value), true
		}
	}

	return "", false
}

// testPath is the PATH test t's runs search: each of its worlds' stand-in
// programs, then this process's own PATH, which TestMain has taken every task
// program off.
func testPath(t *testing.T) string {
	t.Helper()

	dirs := make([]string, 0, len(worldsAbove(t))+1)
	for _, world := range worldsAbove(t) {
		dirs = append(dirs, filepath.Join(world, "bin"))
	}

	return strings.Join(append(dirs, os.Getenv("PATH")), string(os.PathListSeparator))
}

// environmentFor is the Environment a run of test t where says sees: run from
// where.dir, which a switch moves on from, with where.home as its home and none
// of the developer's own environment but what t set.
func environmentFor(t *testing.T, where place) cli.Environment {
	t.Helper()

	getenv := func(name string) string {
		value, _ := lookupVariable(t, where, name)

		return value
	}
	working := &workingDir{dir: where.dir}

	return cli.Environment{
		WorkingDir: working.get,
		Chdir:      working.set,
		Process: wiring.Environment{
			Home:   where.home,
			Getenv: getenv,
			StateDir: func() (string, error) {
				return store.Dir(runtime.GOOS, where.home, func(name string) (string, bool) {
					return lookupVariable(t, where, name)
				})
			},
			LookPath: func(name string) (string, error) { return lookPathIn(getenv("PATH"), name) },
		},
	}
}

// lookupVariable is the variable name as a run of test t where says sees it:
// what t set, or else the isolated environment every run gets.
func lookupVariable(t *testing.T, where place, name string) (string, bool) {
	t.Helper()

	value, set := variableSet(t, name)
	if set {
		return value, true
	}

	if name == "PATH" {
		return testPath(t), true
	}

	value, set = isolatedEnvironment(where)[name]

	return value, set
}

// workingDir is the directory a run is in, which a switch it makes moves.
type workingDir struct {
	lock sync.Mutex
	dir  string
}

// get is the directory the run is in now.
func (w *workingDir) get() (string, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	return w.dir, nil
}

// set moves the run to dir.
func (w *workingDir) set(dir string) error {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.dir = dir

	return nil
}

// lookPathIn finds name on pathList as exec.LookPath finds it on a Unix PATH:
// a name with a slash is taken as it is, and otherwise the first executable
// file of that name in a directory of the list, skipping an empty entry.
func lookPathIn(pathList, name string) (string, error) {
	if strings.Contains(name, "/") {
		if runnable(name) {
			return name, nil
		}

		return "", fmt.Errorf("%w: %s", proc.ErrNotFound, name)
	}

	for _, dir := range filepath.SplitList(pathList) {
		candidate := filepath.Join(dir, name)
		if dir != "" && runnable(candidate) {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%w: %s", proc.ErrNotFound, name)
}

// runnable reports a regular file with an execute bit.
func runnable(path string) bool {
	info, err := os.Stat(path)

	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
