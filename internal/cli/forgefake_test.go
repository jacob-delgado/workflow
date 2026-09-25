// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// ghResponses are the canned bodies a fake gh returns per route; an empty field
// falls back to a benign default (no pull, no CI).
type ghResponses struct {
	pulls       string // GET .../pulls?...              — a JSON array
	status      string // GET .../commits/{sha}/status
	checks      string // GET .../commits/{sha}/check-runs
	search      string // GET .../search/issues?...      — a {"items":[...]} object
	userError   bool   // when set, GET .../user answers unreadably, so Whoami fails
	createError bool   // when set, POST .../pulls answers unreadably, so the open fails
	signedOut   bool   // when set, `gh auth token` exits 1, so no token resolves through gh
	apiHangs    bool   // when set, `gh api` answers nothing for ghHang, as behind a stalled gateway
}

// ghHang is how long a fake gh whose api hangs takes to give up on its own:
// long enough that a test waiting it out cannot be mistaken for one that did
// not.
const ghHang = 10 * time.Second

// openPull is a forge pull request array with one open pull request.
func openPull(title string) string {
	return `[{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"` +
		title + `","state":"open","draft":false}]`
}

// mergedPull is a forge pull request array with one merged pull request — GitHub
// tells a merge from a plain close by a non-null merged_at.
func mergedPull(title string) string {
	return `[{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"` +
		title + `","state":"closed","merged_at":"2026-01-01T00:00:00Z"}]`
}

// failingStatus is a commit status with a failed context.
func failingStatus() string {
	return `{"total_count":1,"statuses":[{"state":"failure","context":"ci","target_url":"https://x"}]}`
}

// fakeGh installs a stand-in `gh` on PATH that answers `gh api --include <url>`
// with canned HTTP responses, so a black-box test drives the forge through its
// CLI transport without a network or a real credential.
func fakeGh(t *testing.T, responses ghResponses) {
	t.Helper()

	dir := t.TempDir()

	// An unreadable body ("{") makes the forge client fail to decode a response,
	// standing in for a route the fake is asked to fail.
	const unreadable = `{`

	create := `{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"work","state":"open"}`
	if responses.createError {
		create = unreadable
	}

	user := `{"login":"octo","name":"Octo Cat"}`
	if responses.userError {
		user = unreadable
	}

	bodies := map[string]string{
		"pulls":   orDefault(responses.pulls, "[]"),
		"create":  create,
		"pull":    `{"mergeable":true}`,
		"reviews": `[]`,
		"status":  orDefault(responses.status, `{"total_count":0,"statuses":[]}`),
		"checks":  orDefault(responses.checks, `{"total_count":0,"check_runs":[]}`),
		"search":  orDefault(responses.search, `{"items":[]}`),
		"user":    user,
		"default": `{}`,
	}
	for name, body := range bodies {
		writeExecutable(t, filepath.Join(dir, "resp-"+name), body, 0o644)
	}

	// A signed-out gh fails `gh auth token` the way the real one does, while its
	// `api` command still answers.
	signIn := ""
	if responses.signedOut {
		signIn = "if [ \"$1\" = auth ]; then exit 1; fi\n"
	}

	// exec hands the pipes to sleep itself, so killing the hung gh closes them
	// rather than leaving a child that holds them open.
	hang := ""
	if responses.apiHangs {
		hang = "if [ \"$1\" = api ]; then exec sleep " + strconv.Itoa(int(ghHang/time.Second)) + "; fi\n"
	}

	script := "#!/bin/sh\n" + signIn + hang +
		"for a in \"$@\"; do url=\"$a\"; done\n" +
		"case \"$url\" in\n" +
		"  *\"/pulls?\"*) f=pulls ;;\n" +
		"  *\"/pulls/\"*\"/reviews\"*) f=reviews ;;\n" +
		"  *\"/pulls/\"*) f=pull ;;\n" +
		"  *\"/pulls\") f=create ;;\n" +
		"  *\"/commits/\"*\"/status\"*) f=status ;;\n" +
		"  *\"/commits/\"*\"/check-runs\"*) f=checks ;;\n" +
		"  *\"/search/issues\"*) f=search ;;\n" +
		"  *\"/user\"*) f=user ;;\n" +
		"  *) f=default ;;\n" +
		"esac\n" +
		"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n'\n" +
		"cat \"" + dir + "/resp-$f\"\n"

	writeExecutable(t, filepath.Join(dir, "gh"), script, 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// gitlabMergeRequest is the merge request a fake glab opens.
const gitlabMergeRequest = "https://gitlab.com/owner/repo/-/merge_requests/7"

// fakeGlab installs a stand-in `glab` on PATH that answers `glab api --include
// <path>` as a GitLab project with no merge request yet, opens one when asked,
// and names its user, so a black-box test drives a GitLab forge through its CLI
// transport.
func fakeGlab(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	bodies := map[string]string{
		"list":    "[]",
		"create":  `{"iid":7,"web_url":"` + gitlabMergeRequest + `","title":"work","state":"opened"}`,
		"user":    `{"username":"tanuki"}`,
		"default": `{}`,
	}

	for name, body := range bodies {
		writeExecutable(t, filepath.Join(dir, "resp-"+name), body, 0o644)
	}

	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do path=\"$a\"; done\n" +
		"case \"$path\" in\n" +
		"  *\"/merge_requests?\"*) f=list ;;\n" +
		"  *\"/merge_requests\") f=create ;;\n" +
		"  user) f=user ;;\n" +
		"  *) f=default ;;\n" +
		"esac\n" +
		"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n'\n" +
		"cat \"" + dir + "/resp-$f\"\n"

	writeExecutable(t, filepath.Join(dir, "glab"), script, 0o755)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// writeRepoFile writes contents to a path under repo, creating parent directories
// — for a repository file the command reads, such as a pull request template.
func writeRepoFile(t *testing.T, repo, rel, contents string) {
	t.Helper()

	path := filepath.Join(repo, rel)

	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}

	writeExecutable(t, path, contents, 0o644)
}

// writeExecutable writes contents to path with mode, failing the test if it
// cannot.
func writeExecutable(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()

	err := os.WriteFile(path, []byte(contents), mode)
	if err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// orDefault is value, or fallback when value is empty.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}

// githubRepo is a repository on branch, one commit ahead of main, with a GitHub
// origin so the forge resolves to gh's dialect.
func githubRepo(t *testing.T, branch string) string {
	t.Helper()

	repo := prRepo(t, branch)
	git(t, repo, "remote", "add", "origin", "https://github.com/owner/repo.git")

	return repo
}

// forgeCLIConfig points the forge at its CLI and configures a Slack webhook, so
// `announce` gets past its Slack guard and reads the pull request through gh.
const forgeCLIConfig = `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
	`"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`

// pretendPushed gives the checked-out branch a local upstream on origin with
// nothing ahead, so the pull-request flow treats it as already published and
// skips the push — there is no real remote to push to in a black-box test.
func pretendPushed(t *testing.T, repo string) {
	t.Helper()

	branch := currentBranch(t, repo)
	git(t, repo, "update-ref", "refs/remotes/origin/"+branch, "HEAD")
	git(t, repo, "branch", "--set-upstream-to=origin/"+branch, branch)
}

// localPushRemote gives repo a bare push remote through remote.pushDefault, so a
// real push in a black-box test lands in a repository on disk rather than
// reaching for the GitHub origin the forge is resolved from. It returns the bare
// repository's path.
func localPushRemote(t *testing.T, repo string) string {
	t.Helper()

	bare := t.TempDir()
	git(t, bare, "init", "--bare", "--quiet")
	git(t, repo, "remote", "add", "push-target", bare)
	git(t, repo, "config", "remote.pushDefault", "push-target")

	return bare
}
