// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

// What doctor says in each row and verdict it gives, compared whole: a test
// that only looks for part of a line passes whatever the rest of it says.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// ran is what one doctor run printed, stdout and stderr together, and what it
// returned, run from dir.
type ran struct {
	output string
	err    error
	dir    string
}

// verdict is the error a run returned, said as a script reads it, or "" for
// none.
func (r ran) verdict() string {
	if r.err == nil {
		return ""
	}

	return r.err.Error()
}

// fixedCase is one setup doctor is run in and the line it is held to.
type fixedCase struct {
	run  func(t *testing.T) ran
	read func(ran) string
	want string
}

// doctorIn runs doctor in dir with args after it.
func doctorIn(t *testing.T, dir string, args ...string) ran {
	t.Helper()

	output, err := clitest.Run(t, dir, append([]string{doctorCommand}, args...)...)

	return ran{output: output, err: err, dir: dir}
}

// onlineWith runs doctor --online in a directory outside any repository
// holding a configuration made of fields.
func onlineWith(fields string) func(t *testing.T) ran {
	return func(t *testing.T) ran {
		t.Helper()

		return doctorIn(t, configuredWith(t, fields), "--online")
	}
}

// onlineIn runs doctor --online in the repository repo makes, with gh or glab
// as onPath puts them.
func onlineIn(repo func(t *testing.T) string, onPath func(t *testing.T)) func(t *testing.T) ran {
	return func(t *testing.T) ran {
		t.Helper()

		dir := repo(t)
		onPath(t)

		return doctorIn(t, dir, "--online")
	}
}

// row reads service's credential row.
func row(service string) func(ran) string {
	return func(r ran) string { return credentialRow(r.output, service) }
}

// tool reads program's tooling row, after its name.
func tool(program string) func(ran) string {
	return func(r ran) string {
		return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(toolingRow(r.output, program)), program))
	}
}

// labeled reads the value doctor printed for label.
func labeled(label string) func(ran) string {
	return func(r ran) string { return fieldValue(r.output, label) }
}

// workingJiraFields are a configuration's Jira, which accepts its token, and
// a webhook.
func workingJiraFields(t *testing.T) string {
	t.Helper()

	return `"jira": {"base_url": "` + clitest.WorkingJira(t) + `", "token": "t"}, ` + clitest.SlackWebhook
}

// gitlabCLIRepository is a GitLab repository with forge.cli on.
func gitlabCLIRepository(t *testing.T) string {
	t.Helper()

	return forgeCLIRepository(t, gitlabSSHRemote)
}

// githubRepository is a GitHub repository whose Jira works.
func githubRepository(t *testing.T) string {
	t.Helper()

	dir := clitest.RepoWithRemote(t, githubSSHRemote)
	clitest.WriteFile(t, dir, jiraTracker(clitest.WorkingJira(t)))

	return dir
}

// onPremisesRepository is a repository on a host that names neither forge,
// whose Jira works.
func onPremisesRepository(t *testing.T) string {
	t.Helper()

	dir := clitest.RepoWithRemote(t, clitest.OnPremisesRemote)
	clitest.WriteFile(t, dir, jiraTracker(clitest.WorkingJira(t)))

	return dir
}

// scopes is a glab whose token has the scopes listed.
func scopes(list string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()

		clitest.FakeGlabWithScopes(t, `{"scopes":[`+list+`]}`)
	}
}

// nothingMore leaves PATH as the test has it.
func nothingMore(*testing.T) {}

// slackDryRun runs doctor --online --dry-run over a Slack login fields keeps.
func slackDryRun(fields string) func(t *testing.T) ran {
	return func(t *testing.T) ran {
		t.Helper()

		return doctorIn(t, configuredWith(t, `"messaging":{"client_id":"1234.5678",`+
			`"client_secret":"client-secret-9999",`+fields+`"channel":"#dev"}`), "--online", "--dry-run")
	}
}

// tokenVariableGivingNothing runs doctor --online from a home whose file has
// Jira's token read from a variable set to nothing.
func tokenVariableGivingNothing(t *testing.T) ran {
	t.Helper()

	clitest.SetVariable(t, clitest.EmptyTokenVariable, "")
	dir := configuredWith(t, `"jira": {"base_url": "`+clitest.WorkingJira(t)+`", "token_env": "`+
		clitest.EmptyTokenVariable+`"}, `+clitest.SlackWebhook)
	output, err := clitest.RunAtHome(t, dir, "doctor", "--online")

	return ran{output: output, err: err}
}

func onlineRowCases() map[string]fixedCase {
	return map[string]fixedCase{
		"jira answers": {
			run: func(t *testing.T) ran {
				t.Helper()

				return onlineWith(workingJiraFields(t))(t)
			},
			read: row("jira"), want: "authenticates as Fred F. User (fred) (token from the configuration file)",
		},
		"no jira": {
			run:  onlineWith(clitest.SlackWebhook),
			read: row("jira"), want: "not configured — the forge's issues are the tracker",
		},
		"a token variable that gives nothing": {
			run:  tokenVariableGivingNothing,
			read: row("jira"), want: "no token from the " + clitest.EmptyTokenVariable + " environment variable",
		},
		"a webhook": {
			run:  onlineWith(clitest.SlackWebhook),
			read: row("slack"), want: "an incoming webhook cannot be checked without posting with it",
		},
		"no remote": {
			run:  onlineWith(clitest.SlackWebhook),
			read: row("forge"), want: "no repository remote, so there is no forge to ask",
		},
		"a host that names no forge": {
			run:  onlineIn(onPremisesRepository, nothingMore),
			read: row("forge"),
			want: "git.example.com is not github.com, a ghe.com tenant or gitlab.com — set forge.kind and forge.host",
		},
		"gh signed out": {
			run:  onlineIn(githubRepository, ghSignedOut),
			read: row("forge"), want: "gh is installed but not signed in to github.com; run `gh auth login`",
		},
		"through gh": {
			run: onlineIn(func(t *testing.T) string {
				t.Helper()

				return forgeCLIRepository(t, githubSSHRemote)
			}, ghSignedOutButAnswering),
			read: row("forge"), want: "authenticates as octo (through gh)",
		},
		"a token that only reads": {
			run:  onlineIn(gitlabCLIRepository, scopes(`"read_api","read_user"`)),
			read: row("forge"),
			want: "authenticates as tanuki (through glab); can read but not write: it has read_api, not api",
		},
		"a token without the API": {
			run:  onlineIn(gitlabCLIRepository, scopes(`"read_user","read_repository"`)),
			read: row("forge"),
			want: "authenticates as tanuki (through glab); cannot use the API: it has read_user, read_repository, not api",
		},
		"a refresh held back": {
			run: slackDryRun(`"refresh_token":"slack-refresh-8888","access_token":"xoxe.xoxp-old",` +
				`"expires_at":"2020-01-01T00:00:00Z",`),
			read: row("slack"),
			want: "the Slack user token is due a refresh, which --dry-run holds back; " +
				"run doctor --online without it to check the token",
		},
		"a login never finished": {
			run:  slackDryRun(""),
			read: row("slack"),
			want: "messaging has no credential: no Slack user token is set up; run `workflow slack login`",
		},
	}
}

func TestDoctorOnlineSaysEachCredentialRowWordForWord(t *testing.T) {
	t.Parallel()

	for name, tt := range onlineRowCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.read(tt.run(t))

			// Assert
			if got != tt.want {
				t.Errorf("doctor's row = %q, want %q", got, tt.want)
			}
		})
	}
}

// withoutGit runs doctor with nothing on PATH, so not even git is found.
func withoutGit(t *testing.T) ran {
	t.Helper()

	clitest.SetVariable(t, "PATH", t.TempDir())

	return doctorIn(t, configuredWith(t, `"jira": {"base_url": "https://jira.example.com", "token": "t"}, `+
		clitest.SlackWebhook))
}

func rowCases() map[string]fixedCase {
	return map[string]fixedCase{
		"git missing": {run: withoutGit, read: tool("git"), want: "MISSING — every repository action runs through it"},
		"no git to read the repository": {
			run: withoutGit, read: labeled("Repository"), want: "(none — git is not on PATH)",
		},
		"forge.cli with no glab": {
			run: func(t *testing.T) ran {
				t.Helper()

				dir := gitlabCLIRepository(t)
				pathWithOnlyGit(t)

				return doctorIn(t, dir)
			},
			read: tool("glab"), want: "not found — forge.cli is on, so GitLab calls go over HTTP with a token instead",
		},
		"a Taskwarrior without the link fields": {
			run: func(t *testing.T) ran {
				t.Helper()

				program := fakeTaskwarrior(t, "3.5.0", "")
				done := doctorIn(t, t.TempDir())

				// The program's path is the test's own; the row is held to the rest.
				return ran{output: strings.ReplaceAll(done.output, program, "PROGRAM"), err: done.err}
			},
			read: tool("taskwarrior"),
			want: "found — 3.5.0 at PROGRAM; add uda.jiraid.type=string, uda.jiraid.label=Jira, " +
				"uda.jiraurl.type=string and uda.jiraurl.label=Jira URL to your taskrc so task jiraid:KEY works " +
				"in your shell",
		},
		"go-task": {
			run: func(t *testing.T) ran {
				t.Helper()

				fakeTaskwarrior(t, "", "")

				return doctorIn(t, t.TempDir())
			},
			read: tool("taskwarrior"),
			want: "not usable — task on PATH is not Taskwarrior (go-task?); set taskwarrior.program, " +
				"e.g. /opt/homebrew/bin/task",
		},
		"a detached HEAD": {
			run: func(t *testing.T) ran {
				t.Helper()

				dir := t.TempDir()
				clitest.GitInit(t, dir)
				clitest.Git(t, dir, "-c", "user.email=test@example.com", "-c", "user.name=Test",
					"commit", "--allow-empty", "--quiet", "--message", "seed")
				clitest.Git(t, dir, "-c", "advice.detachedHead=false", "checkout", "--quiet", "--detach", "HEAD")

				return doctorIn(t, dir)
			},
			read: labeled("Branch"), want: "(detached HEAD — check out a branch before starting work)",
		},
		"no Jira": {
			run: func(t *testing.T) ran {
				t.Helper()

				return doctorIn(t, configuredWith(t, clitest.SlackWebhook))
			},
			read: labeled("Jira"), want: "(not set) (none)",
		},
		"no Jira, as data": {
			run: func(t *testing.T) ran {
				t.Helper()

				printed, err := clitest.RunStreams(t, configuredWith(t, clitest.SlackWebhook),
					clitest.UnusedPrompt(t), "doctor", "--json")

				return ran{output: printed.Stdout, err: err}
			},
			read: func(done ran) string {
				var report struct {
					Configuration struct {
						JiraURL string `json:"jira_url"`
					} `json:"configuration"`
				}

				err := json.Unmarshal([]byte(done.output), &report)
				if err != nil {
					return err.Error()
				}

				return report.Configuration.JiraURL
			},
			want: "(not set)",
		},
		"no home for the store": {
			run: func(t *testing.T) ran {
				t.Helper()

				printed, err := clitest.RunStreamsAt(t, clitest.Place{Dir: t.TempDir(), Home: ""},
					clitest.UnusedPrompt(t), "doctor")

				return ran{output: printed.Stdout, err: err}
			},
			read: labeled("Store"), want: "off (could not determine a data directory for the store)",
		},
	}
}

func TestDoctorSaysEachRowWordForWord(t *testing.T) {
	t.Parallel()

	for name, tt := range rowCases() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := tt.read(tt.run(t))

			// Assert
			if got != tt.want {
				t.Errorf("doctor's row = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDoctorPrintsWhyAFileDoesNotParseBeneathItsRow(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	clitest.WriteFile(t, dir, `{"jira": `)

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor")

	// Assert
	if err == nil || !strings.HasSuffix(printed.Stdout, "Configuration: cannot be parsed\n\n"+err.Error()+"\n") {
		t.Errorf("doctor = %v, want the reason beneath the row, and nothing after it:\n%s", err, printed.Stdout)
	}
}

func TestDoctorPrintsWhyAFileCannotBeReadBeneathItsRow(t *testing.T) {
	t.Parallel()

	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root opens a file whatever its mode, so the sealed one would be read")
	}

	dir := t.TempDir()

	err := os.Chmod(clitest.WriteFile(t, dir, `{}`), 0)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	printed, err := clitest.RunStreams(t, dir, clitest.UnusedPrompt(t), "doctor")

	// Assert
	if err == nil || !strings.HasSuffix(printed.Stdout, "Configuration: cannot be read\n\n"+err.Error()+"\n") {
		t.Errorf("doctor = %v, want the reason beneath the row, and nothing after it:\n%s", err, printed.Stdout)
	}
}

// slackLogged is the request log's line for doctor's question to Slack.
var slackLogged = regexp.MustCompile(`(?m) slack POST /auth\.test \d{3} `)

func TestLogOutlinesDoctorOnlinesSlackCheckUnderSlack(t *testing.T) {
	t.Parallel()

	// Arrange
	clitest.FakeSlack(t, nil)

	dir := t.TempDir()
	clitest.WriteFile(t, dir, clitest.SlackLoggedInConfig())
	logPath := filepath.Join(t.TempDir(), "requests.log")

	// Act
	_, _ = clitest.Run(t, dir, "--request-log", logPath, "doctor", "--online")

	// Assert
	if logged := clitest.ReadLog(t, logPath); !slackLogged.MatchString(logged) {
		t.Errorf("the request log does not outline doctor's check of Slack under slack:\n%s", logged)
	}
}
