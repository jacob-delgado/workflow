// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package wiring_test

// The forge's CLI transport turns a forge request into a `gh`/`glab api`
// invocation and parses the reply. A stand-in gh or glab installed on PATH lets
// a black-box test drive that transport through wiring.Deps — no network, no
// real credential — and see both what the forge answered and what the CLI was
// asked to do.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/wiring"
)

// Shared fixtures for the forge and tracker tests: the host and remote a github
// CLI setup points at, the tracker seams named as strings, and the one forge
// issue transition.
const (
	hostGitHub      = "github.com"
	remoteGitHub    = "https://github.com/owner/repo.git"
	seamSearch      = "Search"
	seamIssue       = "Issue"
	transitionClose = "close"
)

// githubCLIWorkspace is the configuration and workspace that point a forge at
// github.com through its own CLI, the setup every CLI-transport test shares.
func githubCLIWorkspace(t *testing.T) (config.Config, wiring.Workspace) {
	t.Helper()

	cfg := config.Config{Forge: config.Forge{CLI: true, Kind: githubKind, Host: hostGitHub}}

	return cfg, wiring.Workspace{Root: t.TempDir(), Remote: remoteGitHub}
}

// forgeReplies are the canned bodies the fake returns per route; an empty field
// falls back to a benign default. The two bools shape a transport failure
// instead of a route's answer.
type forgeReplies struct {
	create  string // POST .../pulls          — the opened pull request
	search  string // GET .../search/issues    — the issue or review search
	issue   string // GET/PATCH .../issues/{n} — a single issue read or close
	garbage bool   // emit a non-HTTP reply and exit 0 → an unreadable response
	fail    bool   // emit garbage and exit non-zero  → a command failure
}

// forgeCLI is a stand-in gh or glab. It records every invocation's arguments
// and standard input to files the test reads back.
type forgeCLI struct {
	dir string
}

// installForgeCLI puts a stand-in program on PATH answering the forge's API
// routes, and returns a handle to its recordings. A token is placed in the
// environment so the CLI transport is never asked for one through `gh auth
// token`, leaving the fake invoked only for the api calls a test drives.
func installForgeCLI(t *testing.T, program string, replies forgeReplies) *forgeCLI {
	t.Helper()

	dir := t.TempDir()

	defaultCreate := `{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"work","state":"open"}`

	bodies := map[string]string{
		"pulls":   "[]",
		"create":  orDefault(replies.create, defaultCreate),
		"pull":    `{"mergeable":true}`,
		"reviews": "[]",
		"status":  `{"total_count":0,"statuses":[]}`,
		"checks":  `{"total_count":0,"check_runs":[]}`,
		"user":    `{"login":"octo"}`,
		"search":  orDefault(replies.search, `{"items":[]}`),
		"issue":   orDefault(replies.issue, `{"number":42}`),
		"merges":  "[]",
		"default": "{}",
	}
	for name, body := range bodies {
		write(t, filepath.Join(dir, "resp-"+name), body, 0o644)
	}

	write(t, filepath.Join(dir, program), forgeScript(dir, replies), 0o755)

	t.Setenv("GITHUB_TOKEN", "cli-stands-in")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GLAB_TOKEN", "")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return &forgeCLI{dir: dir}
}

// forgeScript is the shell body of the stand-in program: it records its
// arguments and standard input, then either fails as asked or answers the route
// the request's URL names with a well-formed HTTP response.
func forgeScript(dir string, replies forgeReplies) string {
	record := "printf '%s\\n' \"$@\" >> \"" + dir + "/args\"\n" +
		"cat >> \"" + dir + "/stdin\"\n"

	switch {
	case replies.fail:
		return "#!/bin/sh\n" + record +
			"printf 'boom on stderr\\n' >&2\n" +
			"printf 'not http\\n'\n" +
			"exit 3\n"
	case replies.garbage:
		return "#!/bin/sh\n" + record + "printf 'not a valid http response\\n'\n"
	default:
		return "#!/bin/sh\n" + record +
			"for a in \"$@\"; do url=\"$a\"; done\n" +
			"case \"$url\" in\n" +
			"  *\"/search/issues\"*) f=search ;;\n" +
			"  *\"/issues/\"*) f=issue ;;\n" +
			"  *\"/pulls?\"*) f=pulls ;;\n" +
			"  *\"/pulls/\"*\"/reviews\"*) f=reviews ;;\n" +
			"  *\"/pulls/\"*) f=pull ;;\n" +
			"  *\"/pulls\") f=create ;;\n" +
			"  *\"/commits/\"*\"/status\"*) f=status ;;\n" +
			"  *\"/commits/\"*\"/check-runs\"*) f=checks ;;\n" +
			"  *\"/user\"*) f=user ;;\n" +
			"  *\"/merge_requests\"*) f=merges ;;\n" +
			"  *) f=default ;;\n" +
			"esac\n" +
			"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n'\n" +
			"cat \"" + dir + "/resp-$f\"\n"
	}
}

// args are the arguments the fake was last handed, one per recorded line.
func (f *forgeCLI) args() []string {
	data, err := os.ReadFile(filepath.Join(f.dir, "args"))
	if err != nil {
		return nil
	}

	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

// stdin is what the fake read from standard input.
func (f *forgeCLI) stdin() string {
	data, _ := os.ReadFile(filepath.Join(f.dir, "stdin"))

	return string(data)
}

// orDefault is value, or fallback when value is empty.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// containsAll reports whether values all appear in args.
func containsAll(args []string, values ...string) bool {
	for _, value := range values {
		if !slices.Contains(args, value) {
			return false
		}
	}

	return true
}
