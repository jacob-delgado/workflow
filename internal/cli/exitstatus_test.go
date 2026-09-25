// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/httpx"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/loop"
	"github.com/jacob-delgado/workflow/internal/messaging"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// errSomethingElse is a failure no family claims.
var errSomethingElse = errors.New("something else went wrong")

// wantExit fails the test unless err exits with want.
func wantExit(t *testing.T, err error, want int) {
	t.Helper()

	if got := cli.ExitStatus(err); got != want {
		t.Errorf("ExitStatus(%v) = %d, want %d", err, got, want)
	}
}

func TestExitStatusDistinguishesFailureKinds(t *testing.T) {
	cases := map[string]struct {
		err  error
		want int
	}{
		"success":                     {err: nil, want: 0},
		"any other failure":           {err: errSomethingElse, want: 1},
		"not found":                   {err: jira.ErrNotFound, want: 1},
		"a failed push":               {err: loop.PushFailedError{Output: []string{"rejected"}}, want: 1},
		"no way to push":              {err: loop.ErrPushUnavailable, want: 1},
		"missing tooling":             {err: proc.ErrNotFound, want: 1},
		"no configuration":            {err: config.ErrNotFound, want: 3},
		"a configuration unread":      {err: config.ErrInvalid, want: 3},
		"jira refused the token":      {err: jira.ErrUnauthorized, want: 3},
		"jira forbade the token":      {err: jira.ErrForbidden, want: 3},
		"the forge refused a token":   {err: forge.ErrUnauthorized, want: 3},
		"messaging refused a token":   {err: messaging.ErrRejected, want: 3},
		"no jira token":               {err: jira.ErrNoCredential, want: 3},
		"a jira address unusable":     {err: jira.ErrInvalidBaseURL, want: 3},
		"a jira address with a login": {err: jira.ErrCredentialInBaseURL, want: 3},
		"no forge token":              {err: forge.ErrNoToken, want: 3},
		"a forge kind with no host":   {err: forge.ErrKindNeedsHost, want: 3},
		"no messaging credential":     {err: messaging.ErrNoCredential, want: 3},
		"an insecure webhook":         {err: messaging.ErrInsecureWebhook, want: 3},
		"a pull request is open":      {err: loop.PullAlreadyOpenError{}, want: 4},
		"nothing to open":             {err: loop.ErrNothingToOpen, want: 4},
		"no pull request":             {err: loop.ErrNoPullRequest, want: 4},
		"a dirty tree":                {err: loop.ErrDirtyTree, want: 4},
		"nothing staged":              {err: loop.ErrNothingStaged, want: 4},
		"not a repository":            {err: gitrepo.ErrNotARepository, want: 4},
		"jira unreachable":            {err: jira.ErrUnreachable, want: 5},
		"the forge unreachable":       {err: forge.ErrUnreachable, want: 5},
		"messaging unreachable":       {err: messaging.ErrUnreachable, want: 5},
		"rate limited":                {err: httpx.ErrRateLimited, want: 5},
		"interrupted":                 {err: context.Canceled, want: 130},
		"wrapped on the way out": {
			err:  fmt.Errorf("workflow: reading the branch: %w", gitrepo.ErrNotARepository),
			want: 4,
		},
		"interrupted mid-request": {
			err:  httpx.Unreachable(jira.ErrUnreachable, "https://jira.example.com", context.Canceled),
			want: 130,
		},
		"configuration before a refusal": {
			err: errors.Join(loop.ErrDirtyTree, config.ErrNotFound), want: 3,
		},
		"a refusal before unreachable": {
			err: errors.Join(forge.ErrUnreachable, loop.ErrNothingToOpen), want: 4,
		},
		"unreachable before a failure": {
			err: errors.Join(proc.ErrNotFound, jira.ErrUnreachable), want: 5,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Act & Assert
			wantExit(t, tt.err, tt.want)
		})
	}
}

// askJira asks the Jira at base who the token belongs to, over the
// redirect-refusing transport the commands use, and returns what failed.
func askJira(ctx context.Context, base string) error {
	settings := config.Jira{BaseURL: base, Token: "jira-token-for-tests"}
	_, err := jira.New(httpx.Client(time.Second).Do, settings).Myself(ctx)

	return err
}

// postToSlack posts as a Slack bot through the API at base, over the
// redirect-refusing transport the commands use, and returns what failed.
func postToSlack(ctx context.Context, base string) error {
	creds := config.Messaging{Token: "slack-token-for-tests", Channel: "#dev"}

	return messaging.New(httpx.Client(time.Second).Do, base, creds).Post(ctx, "", "a pull request is ready")
}

// askForge asks the forge API at base who the token belongs to, over the
// redirect-refusing transport the commands use, and returns what failed.
func askForge(ctx context.Context, base string) error {
	_, err := forge.New(httpx.Client(time.Second).Do, base, "forge-token-for-tests").Whoami(ctx)

	return err
}

// redirecting answers every request with a redirect to a sign-in page, as a
// gateway in front of a service does.
func redirecting(writer http.ResponseWriter, request *http.Request) {
	http.Redirect(writer, request, "https://sso.example.com/login", http.StatusFound)
}

// answeringWith answers every request with status and body.
func answeringWith(status int, body string) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}
}

func TestAServiceAnswerExitsInItsFamily(t *testing.T) {
	// Each error is the one the service's own client makes of the answer, so
	// the family is the one a command meeting it exits with.
	cases := map[string]struct {
		answer http.HandlerFunc
		ask    func(context.Context, string) error
		want   int
	}{
		"jira refused the token for this, and said why": {
			answer: answeringWith(http.StatusForbidden, `{"errorMessages":["You do not have permission to browse."]}`),
			ask:    askJira, want: 3,
		},
		// Slack answers a post 200 whatever it refused; its code tells the
		// credential from the channel.
		"slack refused a post's token": {
			answer: answeringWith(http.StatusOK, `{"ok":false,"error":"invalid_auth"}`),
			ask:    postToSlack, want: 3,
		},
		"slack refused a post for its channel": {
			answer: answeringWith(http.StatusOK, `{"ok":false,"error":"not_in_channel"}`),
			ask:    postToSlack, want: 1,
		},
		// A redirect is refused so the credential goes nowhere else: the service
		// never answered the request, as a script waiting on 5 expects.
		"jira answered with a redirect":      {answer: redirecting, ask: askJira, want: 5},
		"the forge answered with a redirect": {answer: redirecting, ask: askForge, want: 5},
		"slack answered with a redirect":     {answer: redirecting, ask: postToSlack, want: 5},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			server := httptest.NewServer(tt.answer)
			t.Cleanup(server.Close)

			// Act
			err := tt.ask(t.Context(), server.URL)

			// Assert
			wantExit(t, err, tt.want)
		})
	}
}

func TestExitStatusMarksMisuse(t *testing.T) {
	// Each case names what its message must still say: cobra's words are all a
	// user sees of what was wrong.
	cases := map[string]struct {
		args  string
		names string
	}{
		"an unknown flag":                   {args: "status --no-such-flag", names: "--no-such-flag"},
		"a flag's value unreadable":         {args: "standup --days many", names: `"many"`},
		"an unknown command":                {args: "no-such-command", names: "no-such-command"},
		"an unknown subcommand":             {args: "config no-such-subcommand", names: "no-such-subcommand"},
		"a missing argument":                {args: "branch", names: "accepts 1 arg(s)"},
		"an argument too many":              {args: "reviews extra", names: `"extra"`},
		"an unknown completion shell":       {args: "completion fsh", names: `"fsh"`},
		"an argument to a completion shell": {args: "completion bash surplus", names: `"surplus"`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := run(t, t.TempDir(), strings.Fields(tt.args)...)

			// Assert
			wantExit(t, err, 2)

			if err == nil || !strings.Contains(err.Error(), tt.names) {
				t.Errorf("workflow %s = %v, want a message naming %s", tt.args, err, tt.names)
			}
		})
	}
}

// rootHelp is the pointer at the root's help that a misuse of the root ends with.
const rootHelp = "Run 'workflow --help' for usage."

func TestUnknownCommandPointsAtHelp(t *testing.T) {
	cases := map[string]struct {
		args  string
		meant string
		help  string
	}{
		"a mistyped command":    {args: "brnch PROJ-1", meant: "branch", help: rootHelp},
		"a mistyped subcommand": {args: "config initt", meant: "init", help: "Run 'workflow config --help' for usage."},
		// The flag belongs to the command meant, so it is the name that is wrong.
		"a mistyped command before a flag": {args: "annunce --yes", meant: "announce", help: rootHelp},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), strings.Fields(tt.args)...)

			// Assert
			wantExit(t, err, 2)

			if err == nil || !strings.Contains(err.Error(), "Did you mean this?\n\t"+tt.meant+"\n") ||
				!strings.HasSuffix(err.Error(), tt.help) || printed.stdout != "" {
				t.Errorf("workflow %s = %v, want the closest command, %q, then %q, and nothing on stdout (%q)",
					tt.args, err, tt.meant, tt.help, printed.stdout)
			}
		})
	}
}

func TestMisuseWithNothingToSuggestPointsAtHelpAlone(t *testing.T) {
	cases := map[string]struct {
		args string
		help string
	}{
		"an unknown flag":         {args: "status --no-such-flag", help: "Run 'workflow status --help' for usage."},
		"a name like no command":  {args: "zzzzzz", help: rootHelp},
		"an argument it takes no": {args: "pr extra", help: "Run 'workflow pr --help' for usage."},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := run(t, t.TempDir(), strings.Fields(tt.args)...)

			// Assert
			wantExit(t, err, 2)

			if err == nil || !strings.HasSuffix(err.Error(), tt.help) || strings.Contains(err.Error(), "Did you mean") {
				t.Errorf("workflow %s = %v, want %q and no suggestion", tt.args, err, tt.help)
			}
		})
	}
}

func TestBareConfigPrintsItsHelp(t *testing.T) {
	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "config")

	// Assert
	wantExit(t, err, 0)

	for _, want := range []string{"Available Commands", "init", "show"} {
		if !strings.Contains(printed.stdout, want) {
			t.Errorf("workflow config printed %q, want its help naming %q", printed.stdout, want)
		}
	}
}

func TestCompletionWritesItsScriptToStdout(t *testing.T) {
	// Act
	printed, err := runStreams(t, t.TempDir(), unusedPrompt(t), "completion", "bash")
	// Assert
	if err != nil {
		t.Fatalf("workflow completion bash = %v", err)
	}

	if !strings.Contains(printed.stdout, "bash completion") {
		t.Errorf("workflow completion bash printed %q, want the script on the stdout it was given", printed.stdout)
	}
}

func TestAnInterruptedGitCommandExitsAsInterrupted(t *testing.T) {
	// Arrange
	repo := featureRepo(t)
	// A git that raises Ctrl+C on the process running it and then hangs until it
	// is stopped: the interrupt lands mid-subprocess, where what comes back is
	// git's own exit status rather than the interrupt.
	fakeGit := t.TempDir()
	writeExecutable(t, filepath.Join(fakeGit, "git"), "#!/bin/sh\nkill -INT $PPID\nexec sleep 5\n", 0o755)
	t.Setenv("PATH", fakeGit+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Act
	_, err := run(t, repo, "status")

	// Assert
	wantExit(t, err, 130)
}

func TestEveryCommandRefusesAConfigurationItCannotRead(t *testing.T) {
	// A file that exists but does not parse is not replaced by the defaults: a
	// command run on settings nobody chose would act on the wrong things.
	cases := [][]string{
		strings.Fields("status"),
		strings.Fields("reviews"),
		strings.Fields("standup --no-edit"),
		strings.Fields("branch PROJ-1"),
		strings.Fields("pr --dry-run"),
		strings.Fields("announce --dry-run"),
	}

	for _, args := range cases {
		name := strings.Join(args, " ")
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			writeFile(t, dir, "{not json")

			// Act
			_, err := run(t, dir, args...)

			// Assert
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("%s with a malformed configuration = %v, want it refused", name, err)
			}

			wantExit(t, err, 3)
		})
	}
}

func TestACommandRefusesAConfigurationItCannotOpen(t *testing.T) {
	// Arrange
	if os.Geteuid() == 0 {
		t.Skip("root opens a file whatever its mode, so the sealed one would be read")
	}

	// A repository, so a command falling back to the defaults would succeed
	// rather than fail for some other reason.
	repo := featureRepo(t)
	path := writeFile(t, repo, `{}`)

	err := os.Chmod(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	output, err := run(t, repo, "status")

	// Assert
	if err == nil || errors.Is(err, config.ErrInvalid) || strings.Contains(output, "PROJ-2") {
		t.Errorf("status with a configuration it cannot open = %v, want it refused as unopened:\n%s", err, output)
	}

	wantExit(t, err, 1)
}

func TestExitStatusPutsMisuseBeforeEveryOtherKind(t *testing.T) {
	// Arrange
	_, misuse := run(t, t.TempDir(), "status", "--no-such-flag")

	// Act & Assert
	wantExit(t, errors.Join(config.ErrNotFound, misuse), 2)
}

func TestExitStatusPutsTheConfigurationBeforeMissingTooling(t *testing.T) {
	// Arrange
	// No PATH, so git is missing, and no configuration either: doctor reports
	// both, and the configuration is what a script should act on first.
	t.Setenv("PATH", "")

	// Act
	_, err := run(t, t.TempDir(), "doctor")

	// Assert
	wantExit(t, err, 3)
}

func TestExitStatusOfMissingToolingAlone(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.com", "token": "t"},`+
		` "messaging": {"webhook_url": "https://hooks.slack.example/services/not-real"}}`)
	t.Setenv("PATH", "")

	// Act
	_, err := run(t, dir, "doctor")

	// Assert
	wantExit(t, err, 1)
}
