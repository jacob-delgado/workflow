// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package clitest

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// GhResponses are the canned bodies a fake gh returns per route; an empty field
// falls back to a benign default (no pull, no CI).
type GhResponses struct {
	Pulls  string // GET .../pulls?...              — a JSON array
	Status string // GET .../commits/{sha}/status
	// StatusSequence and pullsSequence, when set, answer the status reads and
	// the pull request lists in turn, the last one repeated, in place of
	// status and pulls.
	StatusSequence []string
	PullsSequence  []string
	// StatusGate, when set, is a directory: each status read touches its read
	// file, and answers running until its release file is there, then passed.
	StatusGate  string
	Checks      string // GET .../commits/{sha}/check-runs
	Search      string // GET .../search/issues?...      — a {"items":[...]} object
	UserError   bool   // when set, GET .../user answers unreadably, so Whoami fails
	CreateError bool   // when set, POST .../pulls answers unreadably, so the open fails
	SignedOut   bool   // when set, `gh auth token` exits 1, so no token resolves through gh
	APIHangs    bool   // when set, `gh api` answers nothing for GhHang, as behind a stalled gateway
	// InterruptsStatus, when set, has each status read press ctrl+c on the
	// process that asked, as a person would mid-wait, and then hang.
	InterruptsStatus bool
	SentLog          string // when set, each request carrying a body appends its endpoint and body here
}

// The modes the fakes write their files with: a body read, and a program run;
// and the mode of a directory a repository's file is written into.
const (
	dataMode    = 0o644
	programMode = 0o755
	dirMode     = 0o750
)

// The routes a fake gh answers from a sequence as well as from one body, and
// the one a fake glab does.
const (
	statusRoute = "status"
	pullsRoute  = "pulls"
	mergeRoute  = "merge"
)

// GhHang is how long a fake gh whose api hangs takes to give up on its own:
// long enough that a test waiting it out cannot be mistaken for one that did
// not.
const GhHang = 10 * time.Second

// OpenPull is a forge pull request array with one open pull request.
func OpenPull(title string) string {
	return `[{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"` +
		title + `","state":"open","draft":false}]`
}

// MergedPull is a forge pull request array with one merged pull request — GitHub
// tells a merge from a plain close by a non-null merged_at.
func MergedPull(title string) string {
	return `[{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"` +
		title + `","state":"closed","merged_at":"2026-01-01T00:00:00Z"}]`
}

// ClosedPull is a forge pull request array with one pull request closed
// without merging: its merged_at is null.
func ClosedPull(title string) string {
	return `[{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"` +
		title + `","state":"closed","merged_at":null}]`
}

// FailingStatus is a commit status with a failed context.
func FailingStatus() string {
	return `{"total_count":1,"statuses":[{"state":"failure","context":"ci","target_url":"https://x"}]}`
}

// FakeGh installs a stand-in `gh` first on the test's PATH that answers `gh api --include <url>`
// with canned HTTP responses, so a black-box test drives the forge through its
// CLI transport without a network or a real credential.
func FakeGh(t *testing.T, responses GhResponses) {
	t.Helper()

	dir := t.TempDir()
	writeGhBodies(t, dir, responses)

	WriteExecutable(t, filepath.Join(ProgramsOf(t), "gh"), ghScript(dir, responses), programMode)
}

// writeGhBodies writes into dir the body a fake gh answering responses
// answers each route with, and each body of a sequence.
func writeGhBodies(t *testing.T, dir string, responses GhResponses) {
	t.Helper()

	// An unreadable body ("{") makes the forge client fail to decode a response,
	// standing in for a route the fake is asked to fail.
	const unreadable = `{`

	create := `{"number":7,"html_url":"https://github.com/owner/repo/pull/7","title":"work","state":"open"}`
	if responses.CreateError {
		create = unreadable
	}

	user := `{"login":"octo","name":"Octo Cat"}`
	if responses.UserError {
		user = unreadable
	}

	bodies := map[string]string{
		pullsRoute:       orDefault(responses.Pulls, "[]"),
		"create":         create,
		"pull":           `{"mergeable":true}`,
		"reviews":        `[]`,
		statusRoute:      orDefault(responses.Status, `{"total_count":0,"statuses":[]}`),
		"checks":         orDefault(responses.Checks, `{"total_count":0,"check_runs":[]}`),
		"search":         orDefault(responses.Search, `{"items":[]}`),
		"user":           user,
		"default":        `{}`,
		"status-running": RunningStatus(),
		"status-passed":  PassingStatus(),
	}
	for name, body := range bodies {
		WriteExecutable(t, filepath.Join(dir, "resp-"+name), body, dataMode)
	}

	sequences := map[string][]string{statusRoute: responses.StatusSequence, pullsRoute: responses.PullsSequence}
	for route, sequence := range sequences {
		for index, body := range sequence {
			WriteExecutable(t, filepath.Join(dir, "resp-"+route+"-"+strconv.Itoa(index)), body, dataMode)
		}
	}
}

// ghScript is the fake gh answering responses from the bodies in dir.
func ghScript(dir string, responses GhResponses) string {
	// A signed-out gh fails `gh auth token` the way the real one does, while its
	// `api` command still answers.
	signIn := ""
	if responses.SignedOut {
		signIn = "if [ \"$1\" = auth ]; then exit 1; fi\n"
	}

	// exec hands the pipes to sleep itself, so killing the hung gh closes them
	// rather than leaving a child that holds them open.
	hang := ""
	if responses.APIHangs {
		hang = "if [ \"$1\" = api ]; then exec sleep " + strconv.Itoa(int(GhHang/time.Second)) + "; fi\n"
	}

	// The parent is the test's own process, which runs the command in it.
	interrupt := ""
	if responses.InterruptsStatus {
		interrupt = "if [ \"$f\" = " + statusRoute + " ]; then kill -INT $PPID; exec sleep 5; fi\n"
	}

	// A request with a body reads it from standard input (--input -).
	record := ""
	if responses.SentLog != "" {
		record = "case \" $* \" in *\" --input \"*) { echo \"$url\"; cat; echo; } >> \"" + responses.SentLog + "\" ;; esac\n"
	}

	return "#!/bin/sh\n" + signIn + hang +
		"for a in \"$@\"; do url=\"$a\"; done\n" + record +
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
		interrupt +
		gated(responses.StatusGate) +
		sequenced(dir, statusRoute, len(responses.StatusSequence)) +
		sequenced(dir, pullsRoute, len(responses.PullsSequence)) +
		"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n'\n" +
		"cat \"" + dir + "/resp-$f\"\n"
}

// gated is the fake gh's step that answers the status reads by gate's
// release file, touching its read file; nothing when there is no gate.
func gated(gate string) string {
	if gate == "" {
		return ""
	}

	return "if [ \"$f\" = " + statusRoute + " ]; then\n" +
		"  touch \"" + gate + "/read\"\n" +
		"  if [ -f \"" + gate + "/release\" ]; then f=status-passed; else f=status-running; fi\n" +
		"fi\n"
}

// sequenced is a fake's step that answers the reads of route from its
// sequence, each read the next, the last of count repeated; nothing when
// there is none.
func sequenced(dir, route string, count int) string {
	if count == 0 {
		return ""
	}

	last, reads := strconv.Itoa(count-1), dir+"/"+route+"-reads"

	return "if [ \"$f\" = " + route + " ]; then\n" +
		"  n=$(cat \"" + reads + "\" 2>/dev/null || echo 0)\n" +
		"  if [ \"$n\" -gt " + last + " ]; then n=" + last + "; fi\n" +
		"  echo $((n + 1)) > \"" + reads + "\"\n" +
		"  f=" + route + "-$n\n" +
		"fi\n"
}

// GitlabMergeRequest is the merge request a fake glab opens.
const GitlabMergeRequest = "https://gitlab.com/owner/repo/-/merge_requests/7"

// FakeGlab installs a stand-in `glab` on PATH that answers `glab api --include
// <path>` as a GitLab project with no merge request yet, opens one when asked,
// and names its user and no other, so a black-box test drives a GitLab forge through its CLI
// transport. Its token's scopes are not told.
func FakeGlab(t *testing.T) {
	t.Helper()

	FakeGlabWithScopes(t, `{}`)
}

// FakeGlabWithScopes is FakeGlab whose token, asked about, answers scopes — the
// body of GitLab's personal_access_tokens/self.
func FakeGlabWithScopes(t *testing.T, scopes string) {
	t.Helper()

	fakeGlabAnswering(t, "[]", scopes)
}

// FakeGlabListing is FakeGlab whose merge-request listings — the branch's own
// and the review queue's alike — answer list.
func FakeGlabListing(t *testing.T, list string) {
	t.Helper()

	fakeGlabAnswering(t, list, `{}`)
}

// FakeGlabMerging is FakeGlab whose merge-request listings answer list and
// whose merge request 7, read on its own as its pipeline is, answers each of
// merges in turn, the last repeated.
func FakeGlabMerging(t *testing.T, list string, merges ...string) {
	t.Helper()

	fakeGlabAnswering(t, list, `{}`, merges...)
}

// fakeGlabAnswering is FakeGlab whose listings answer list, whose token, asked
// about, answers scopes, and whose merge request 7, read on its own, answers
// merges in turn, or nothing it knows of when there are none.
func fakeGlabAnswering(t *testing.T, list, scopes string, merges ...string) {
	t.Helper()

	dir := t.TempDir()
	bodies := map[string]string{
		mergeRoute: `{}`,
		"list":     list,
		"create":   `{"iid":7,"web_url":"` + GitlabMergeRequest + `","title":"work","state":"opened"}`,
		"user":     `{"username":"tanuki"}`,
		"users":    "[]",
		"scopes":   scopes,
		"default":  `{}`,
	}

	for name, body := range bodies {
		WriteExecutable(t, filepath.Join(dir, "resp-"+name), body, dataMode)
	}

	for index, body := range merges {
		WriteExecutable(t, filepath.Join(dir, "resp-"+mergeRoute+"-"+strconv.Itoa(index)), body, dataMode)
	}

	script := "#!/bin/sh\n" +
		"for a in \"$@\"; do path=\"$a\"; done\n" +
		"case \"$path\" in\n" +
		"  *\"/merge_requests?\"* | \"merge_requests?\"*) f=list ;;\n" +
		"  *\"/merge_requests\") f=create ;;\n" +
		"  *\"/merge_requests/7\") f=" + mergeRoute + " ;;\n" +
		"  user) f=user ;;\n" +
		"  *\"users?\"*) f=users ;;\n" +
		"  personal_access_tokens/self) f=scopes ;;\n" +
		"  *) f=default ;;\n" +
		"esac\n" +
		sequenced(dir, mergeRoute, len(merges)) +
		"printf 'HTTP/1.1 200 OK\\r\\nContent-Type: application/json\\r\\n\\r\\n'\n" +
		"cat \"" + dir + "/resp-$f\"\n"

	WriteExecutable(t, filepath.Join(ProgramsOf(t), "glab"), script, programMode)
}

// WriteExecutable writes contents to path with mode, failing the test if it
// cannot. It holds syscall.ForkLock while the file is open: a program another
// test starts meanwhile would inherit the open file and hold it for writing
// until it execs, and running a script written here would then fail with
// "text file busy" before the script ran (golang.org/issue/22315).
func WriteExecutable(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()

	syscall.ForkLock.Lock()
	err := os.WriteFile(path, []byte(contents), mode)
	syscall.ForkLock.Unlock()

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

// ForgeCLIConfig points the forge at its CLI and configures a Slack webhook, so
// `announce` gets past its Slack guard and reads the pull request through gh.
const ForgeCLIConfig = `{"forge":{"cli":true,"kind":"github","host":"github.com"},` +
	`"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`

// RunningStatus is a commit status still to finish.
func RunningStatus() string {
	return `{"total_count":1,"statuses":[{"state":"pending","context":"ci","target_url":"https://x"}]}`
}

// PassingStatus is a commit status that has passed.
func PassingStatus() string {
	return `{"total_count":1,"statuses":[{"state":"success","context":"ci","target_url":"https://x"}]}`
}

// GitlabForgeConfig points the forge at GitLab's CLI, with a webhook so
// announce gets past its messaging guard.
const GitlabForgeConfig = `{"forge":{"cli":true,"kind":"gitlab","host":"gitlab.com"},` +
	`"messaging":{"webhook_url":"https://hooks.slack.example/services/x"}}`

// OpenMergeRequest is a listing holding merge request 7, open, as GitLab sends
// it to both the branch's lookup and the review queue.
func OpenMergeRequest() string {
	return `[{"iid":7,"web_url":"` + GitlabMergeRequest + `","title":"fix: token","state":"opened",` +
		`"created_at":"` + time.Now().Add(-50*time.Hour).UTC().Format(time.RFC3339) + `",` +
		`"author":{"username":"ben"},"references":{"full":"grp/proj!7"}}]`
}
