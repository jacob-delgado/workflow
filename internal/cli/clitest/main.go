// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"encoding/pem"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/gittest"
)

// Main runs the tests m holds in a process readied for them and returns their
// exit code, for a TestMain to exit with once it has cleared git's repository
// variables (gittest.ClearLocalVariables): it trusts the local servers the
// tests deliver to, for as long as they run, takes every task program off
// PATH, and keeps every program a test or a command starts out of the
// developer's own home and git configuration; once they have run, it removes
// what it made, every test's world with it.
func Main(m *testing.M) int {
	roots := trustTestServers()
	shadows := withoutTaskPrograms()
	home := awayFromHome()
	code := m.Run()

	for _, removed := range append(shadows, roots, home, worldsRoot()) {
		err := os.RemoveAll(removed)
		if err != nil {
			panic(err)
		}
	}

	return code
}

// awayFromHome points this process, and so every program it starts, at a
// home of its own, and has git read no configuration but a repository's own
// and the tests' (gittest.Configure), which it writes there — a developer's
// global commit.gpgsign would fail a commit, and maintenance git left running
// could write into a test's repository as it is removed — and returns the home
// it made. Each run is handed a home of its own; this keeps git, and whatever
// else the programs a run starts read, off the developer's.
func awayFromHome() string {
	home, err := os.MkdirTemp("", "workflow-test-home-*")
	if err != nil {
		panic(err)
	}

	isolated := map[string]string{
		"HOME": home, "XDG_STATE_HOME": "", "XDG_CONFIG_HOME": "", "AppData": filepath.Join(home, "AppData"),
	}

	err = gittest.Configure(home, os.Setenv)
	for name, value := range isolated {
		err = errors.Join(err, os.Setenv(name, value))
	}

	if err != nil {
		panic(err)
	}

	return home
}

// withoutTaskPrograms takes every task program off PATH, and returns the
// directories it made to do so. doctor looks for Taskwarrior among them, and a
// test must never run the machine's own: go-task, or the user's Taskwarrior over
// their own tasks. A directory holding one is replaced by a directory of links to
// everything else in it, since git may live beside it.
func withoutTaskPrograms() []string {
	var (
		path    []string
		shadows []string
	)

	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if !holdsTaskProgram(dir) {
			path = append(path, dir)

			continue
		}

		absolute, err := filepath.Abs(dir)
		if err != nil {
			panic(err)
		}

		shadow := linksToAllButTask(absolute)
		path = append(path, shadow)
		shadows = append(shadows, shadow)
	}

	err := os.Setenv("PATH", strings.Join(path, string(os.PathListSeparator)))
	if err != nil {
		panic(err)
	}

	return shadows
}

// holdsTaskProgram reports a directory with a task or task.exe in it.
func holdsTaskProgram(dir string) bool {
	for _, name := range []string{"task", "task.exe"} {
		_, err := os.Stat(filepath.Join(dir, name)) //nolint:gosec // a directory of this process's own PATH
		if err == nil {
			return true
		}
	}

	return false
}

// linksToAllButTask is a new directory linking to every entry of dir but its
// task program.
func linksToAllButTask(dir string) string {
	shadow, err := os.MkdirTemp("", "workflow-test-path-*")
	if err != nil {
		panic(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), "task") || strings.EqualFold(entry.Name(), "task.exe") {
			continue
		}

		err = os.Symlink(filepath.Join(dir, entry.Name()), filepath.Join(shadow, entry.Name()))
		if err != nil {
			panic(err)
		}
	}

	return shadow
}

// trustTestServers makes the certificate every httptest TLS server presents
// the only one the process trusts, so a command can deliver to a local webhook
// over https, the one scheme the messaging client accepts. SSL_CERT_FILE names
// it, which macOS honors as Linux does since Go 1.27; the system reads its
// roots once, at the first connection, so this runs before any test. It
// returns the file it wrote, for removing once the tests are done.
func trustTestServers() string {
	server := httptest.NewTLSServer(nil)
	certificate := server.Certificate()
	server.Close()

	file, err := os.CreateTemp("", "workflow-test-roots-*.pem")
	if err != nil {
		panic(err)
	}

	err = pem.Encode(file, &pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	if err != nil {
		panic(err)
	}

	err = file.Close()
	if err != nil {
		panic(err)
	}

	err = os.Setenv("SSL_CERT_FILE", file.Name())
	if err != nil {
		panic(err)
	}

	return file.Name()
}
