// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/cli"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// What the stand-in interface and web server write to the stream each is
// handed, so a test can tell which stream that was, and what the command's
// input holds, so a test can tell which input the interface read.
const (
	interfaceWrote = "interface"
	serverWrote    = "server"
	keysTyped      = "keys typed"
)

// rootRun is one run of the root command over a stand-in interface and web
// server: how often it started each and what it handed them, where it asked
// the server to serve, what it said on stdout and stderr, and how it ended.
type rootRun struct {
	interfaces int
	model      tui.Model
	// models are every interface opened, in order; nexts are where each in
	// turn asks to go when it ends, none past the last.
	models []tui.Model
	nexts  []tui.Next
	// read is what the interface read from the input it was handed.
	read string
	// env is the Environment the run was handed, which a switch moves on.
	env     cli.Environment
	servers int
	addr    string
	cfg     config.Config
	deps    webserver.Deps
	info    webserver.Info
	stdout  string
	stderr  string
	err     error
}

// runInterface stands in for tui.Run, keeping the model it was handed and
// what it read from its input, and writing interfaceWrote where the interface
// draws, then ending where nexts says, in turn.
func (r *rootRun) runInterface(_ context.Context, model tui.Model, in io.Reader, out io.Writer) (tui.Next, error) {
	r.interfaces++
	r.model = model
	r.models = append(r.models, model)

	read, err := io.ReadAll(in)
	if err != nil {
		return tui.Next{}, fmt.Errorf("reading the interface's input: %w", err)
	}

	r.read = string(read)

	fmt.Fprint(out, interfaceWrote)

	if len(r.nexts) == 0 {
		return tui.Next{}, nil
	}

	next := r.nexts[0]
	r.nexts = r.nexts[1:]

	return next, nil
}

// serveWebAt stands in for WebServerAt, keeping the address it was handed.
func (r *rootRun) serveWebAt(addr string) cli.RunWeb {
	r.addr = addr

	return r.serveWeb
}

// serveWeb stands in for the web server, keeping the configuration and the
// facts about the run it was handed and writing serverWrote to its notes.
func (r *rootRun) serveWeb(
	_ context.Context, cfg config.Config, deps webserver.Deps, info webserver.Info, notes io.Writer,
) error {
	r.servers++
	r.cfg = cfg
	r.deps = deps
	r.info = info

	fmt.Fprint(notes, serverWrote)

	return nil
}

// spine is the top line of the interface the root command opened, as the
// user would read it.
func (r *rootRun) spine() string {
	return strings.SplitN(ansi.Strip(r.model.View().Content), "\n", 2)[0]
}

// huesDrawn are the system hues — the red, green, yellow, blue and magenta
// foregrounds — in the interface the root command opened.
func (r *rootRun) huesDrawn() []string {
	view := r.model.View().Content

	var drawn []string

	for _, hue := range []string{"\x1b[31m", "\x1b[32m", "\x1b[33m", "\x1b[34m", "\x1b[35m"} {
		if strings.Contains(view, hue) {
			drawn = append(drawn, hue)
		}
	}

	return drawn
}

// runRoot runs the root command in dir over a stand-in interface and web
// server, in an empty home of its own. Like run, it hands the run an
// Environment of its own.
func runRoot(t *testing.T, dir string, args ...string) *rootRun {
	t.Helper()

	return runRootAt(t, place{dir: dir, home: t.TempDir()}, args...)
}

// runRootAt is runRoot in the home directory the test chose.
func runRootAt(t *testing.T, where place, args ...string) *rootRun {
	t.Helper()

	return runRootSwitching(t, where, nil, args...)
}

// runRootSwitching is runRootAt over interfaces that end asking to go where
// nexts says, in turn.
func runRootSwitching(t *testing.T, where place, nexts []tui.Next, args ...string) *rootRun {
	t.Helper()

	ran := rootRun{nexts: nexts, env: environmentFor(t, where)}

	ran.stdout, ran.stderr, ran.err = executeRootIn(t, ran.env, ran.runInterface, ran.serveWebAt, args...)

	return &ran
}

// workingDir is the directory the run is in now, after any switch it made.
func (r *rootRun) workingDir() string {
	dir, _ := r.env.WorkingDir()

	return dir
}

// executeRoot runs the root command where the test chose over the interface
// and web server given, and returns what it said on stdout and on stderr, and
// how it ended.
func executeRoot(
	t *testing.T, where place, run cli.RunInterface, serveAt cli.RunWebAt, args ...string,
) (string, string, error) {
	t.Helper()

	return executeRootIn(t, environmentFor(t, where), run, serveAt, args...)
}

// executeRootIn is executeRoot in env.
func executeRootIn(
	t *testing.T, env cli.Environment, run cli.RunInterface, serveAt cli.RunWebAt, args ...string,
) (string, string, error) {
	t.Helper()

	var stdout, stderr bytes.Buffer

	root := cli.NewRootCmdOver(unusedPrompt(t), run, serveAt, env)
	root.SetArgs(args)
	root.SetIn(strings.NewReader(keysTyped))
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.ExecuteContext(t.Context())

	return stdout.String(), stderr.String(), err
}

func TestBareWorkflowOpensTheInterfaceWithItsWritesLive(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir())

	// Assert
	if ran.err != nil || ran.interfaces != 1 || ran.servers != 0 {
		t.Fatalf("workflow = %v, opened %d interfaces and %d servers; want the interface alone",
			ran.err, ran.interfaces, ran.servers)
	}

	if spine := ran.spine(); strings.Contains(spine, "DRY RUN") {
		t.Errorf("the interface opened as a dry run without --dry-run: %q", spine)
	}

	if !strings.Contains(ran.stdout, interfaceWrote) || strings.Contains(ran.stderr, interfaceWrote) {
		t.Errorf("the interface wrote to stdout %q and stderr %q, want its writes on stdout", ran.stdout, ran.stderr)
	}
}

func TestTheInterfaceReadsTheCommandsInput(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir())

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if ran.read != keysTyped {
		t.Errorf("the interface read %q, want the command's input %q", ran.read, keysTyped)
	}
}

func TestDryRunOpensTheInterfaceHoldingItsWritesBack(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir(), "--dry-run")

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow --dry-run = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if spine := ran.spine(); !strings.Contains(spine, "DRY RUN") {
		t.Errorf("the interface's spine = %q, want it to say DRY RUN", spine)
	}
}

// storeKept is where a run with home as its home keeps the store, and whether
// its directory exists there.
func storeKept(t *testing.T, home string) (string, bool) {
	t.Helper()

	dir := storeDirIn(t, home)
	_, err := os.Stat(dir)

	return dir, err == nil
}

func TestADryRunInterfaceLeavesNoStoreOnDisk(t *testing.T) {
	t.Parallel()

	// Arrange
	// With a Jira to key it by, the interface would seed its issue list from any
	// store it is handed. Nothing on disk shows only that none was made; that a
	// dry run drops the store is TestADryRunInterfaceOpensWithoutTheKeptIssueList.
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net"}}`)
	where := place{dir: dir, home: t.TempDir()}

	// Act
	ran := runRootAt(t, where, "--dry-run")

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow --dry-run = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if kept, found := storeKept(t, where.home); found {
		t.Errorf("workflow --dry-run made the store at %s, want nothing on disk", kept)
	}
}

// The twin of the test above, so its Assert is seen to fail when the store is
// opened: the same interface, its writes live, opens the store to seed its list.
func TestTheInterfaceOpensTheStoreToSeedItsIssueList(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net"}}`)
	where := place{dir: dir, home: t.TempDir()}

	// Act
	ran := runRootAt(t, where)

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if kept, found := storeKept(t, where.home); !found {
		t.Errorf("workflow opened no store at %s, want its issue list seeded from it", kept)
	}
}

func TestColorTurnedOffOpensTheInterfaceWithoutHues(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		noColor       string
		configuration string
	}{
		"NO_COLOR set":   {noColor: "1", configuration: `{}`},
		"ui.color never": {noColor: "", configuration: `{"ui": {"color": "never"}}`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			writeFile(t, dir, tt.configuration)
			setVariable(t, "NO_COLOR", tt.noColor)

			// Act
			ran := runRoot(t, dir)

			// Assert
			if ran.err != nil || ran.interfaces != 1 {
				t.Fatalf("workflow = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
			}

			if hues := ran.huesDrawn(); len(hues) != 0 {
				t.Errorf("the interface opened with color off draws the hues %q, want none", hues)
			}
		})
	}
}

// The twin of the test above, so its Assert is seen to fail when the hues are
// drawn: the same interface, with color left on, draws them.
func TestColorLeftOnOpensTheInterfaceWithItsHues(t *testing.T) {
	t.Parallel()

	// Arrange
	setVariable(t, "NO_COLOR", "")

	// Act
	ran := runRoot(t, t.TempDir())

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if hues := ran.huesDrawn(); len(hues) == 0 {
		t.Errorf("the interface opened with color on draws no hue:\n%q", ran.model.View().Content)
	}
}

func TestAConflictingKeymapStopsTheInterfaceBeforeItOpens(t *testing.T) {
	t.Parallel()

	// Arrange
	// commit and stage-all both live on the Branch and Commits panes, so binding
	// commit to stage-all's key is a conflict.
	dir := t.TempDir()
	writeFile(t, dir, `{"ui": {"keys": {"commit": "a"}}}`)

	// Act
	ran := runRoot(t, dir)

	// Assert
	if ran.err == nil || !strings.Contains(ran.err.Error(), "stage-all") {
		t.Errorf("workflow = %v, want the key conflict reported", ran.err)
	}

	if ran.interfaces != 0 {
		t.Errorf("the interface opened %d times over a conflicting keymap, want never", ran.interfaces)
	}
}

func TestARefusedKeymapExitsAsAConfigurationProblem(t *testing.T) {
	t.Parallel()

	// Each map is one the file's owner fixes in the file, as doctor counts it.
	keymaps := map[string]string{
		"an action that does not exist": `{"no-such-action": "C"}`,
		"two actions on one key":        `{"commit": "a"}`,
		"an action no one key can move": `{"jump-to-pane": "f12"}`,
		"interrupt on a key that types": `{"interrupt": "x"}`,
		"a text field's key on an edit": `{"worktree": "ctrl+w"}`,
	}

	for name, keymap := range keymaps {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			writeFile(t, dir, `{"ui": {"keys": `+keymap+`}}`)

			// Act
			ran := runRoot(t, dir)

			// Assert
			if got := cli.ExitStatus(ran.err); got != 3 {
				t.Errorf("workflow over ui.keys %s exits %d (%v), want 3", keymap, got, ran.err)
			}
		})
	}
}

func TestTheWebFlagServesTheLoadedConfigurationInsteadOfTheInterface(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net"}}`)

	// Act
	ran := runRoot(t, dir, "--web")

	// Assert
	if ran.err != nil || ran.servers != 1 || ran.interfaces != 0 {
		t.Fatalf("workflow --web = %v, opened %d servers and %d interfaces; want the server alone",
			ran.err, ran.servers, ran.interfaces)
	}

	if ran.cfg.Jira.BaseURL != "https://jira.example.net" {
		t.Errorf("the server was handed Jira at %q, want the directory's configuration", ran.cfg.Jira.BaseURL)
	}

	if ran.info.Version != buildinfo.Current() || ran.info.DryRun {
		t.Errorf("the server was told %+v, want this build's version and its writes live", ran.info)
	}

	if ran.stderr != serverWrote || ran.stdout != "" {
		t.Errorf("workflow --web said %q on stderr and %q on stdout, want only the server's notes, on stderr",
			ran.stderr, ran.stdout)
	}
}

func TestTheWebFlagCarriesDryRunToTheServer(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir(), "--web", "--dry-run")

	// Assert
	if ran.err != nil || ran.servers != 1 || !ran.info.DryRun {
		t.Errorf("workflow --web --dry-run = %v, served %d times, told %+v; want the server told to hold writes back",
			ran.err, ran.servers, ran.info)
	}
}

func TestTheWebFlagTellsTheServerTheTaskwarriorSettingsItStartedWith(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"taskwarrior": {"program": "/opt/homebrew/bin/task", "disabled": true}}`)

	// Act
	ran := runRootAt(t, place{dir: dir, home: dir}, "--web")

	// Assert
	want := config.Taskwarrior{Program: "/opt/homebrew/bin/task", Disabled: true}
	if ran.err != nil || ran.info.Taskwarrior != want {
		t.Errorf("workflow --web = %v, told the server %+v; want the Taskwarrior settings %+v it started with",
			ran.err, ran.info, want)
	}
}

func TestTheWebFlagSaysWhyTheConfigurationDidNotLoadAndServesAnyway(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": `)

	// Act
	ran := runRoot(t, dir, "--web")

	// Assert
	if ran.err != nil || ran.servers != 1 {
		t.Fatalf("workflow --web = %v, served %d times; want the server started to explain it", ran.err, ran.servers)
	}

	if !strings.Contains(ran.stderr, "configuration did not load cleanly") {
		t.Errorf("workflow --web said %q, want why the configuration did not load", ran.stderr)
	}
}

func TestTheWebFlagWithNoFileNamesTheWaysToSetOneUp(t *testing.T) {
	t.Parallel()

	// Act
	ran := runRoot(t, t.TempDir(), "--web")

	// Assert
	if ran.err != nil || ran.servers != 1 {
		t.Fatalf("workflow --web = %v, served %d times; want the server started to set one up", ran.err, ran.servers)
	}

	for _, want := range []string{config.NoConfigHeadline, "workflow config init", "Settings"} {
		if !strings.Contains(ran.stderr, want) {
			t.Errorf("workflow --web with no file said %q, want %q", ran.stderr, want)
		}
	}

	if strings.Contains(ran.stderr, "did not load cleanly") {
		t.Errorf("workflow --web called no file a load failure: %q", ran.stderr)
	}
}

func TestTheWebServerSaysWhereItServesAndStopsWithItsRun(t *testing.T) {
	t.Parallel()

	// Arrange
	// Port 0 takes any free port, so the test never meets a workflow already
	// serving on the loopback address; the run is canceled before it starts.
	const addr = "127.0.0.1:0"

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var notes bytes.Buffer

	// Act
	err := cli.WebServerAt(addr)(ctx, config.Default(), webserver.Deps{}, webserver.Info{}, &notes)
	// Assert
	if err != nil {
		t.Errorf("a canceled web server returned %v, want it stopped cleanly", err)
	}

	if !strings.Contains(notes.String(), "serving http://127.0.0.1:") || strings.Contains(notes.String(), addr+"/") {
		t.Errorf("the web server said %q, want the loopback port it bound", notes.String())
	}
}

func TestTheWebServerRefusesAConfigurationPathItCannotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The configuration's path is a directory, so its revision cannot be read.
	cfg := config.Default()
	cfg.Path = t.TempDir()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var notes bytes.Buffer

	// Act
	err := cli.WebServerAt("127.0.0.1:0")(ctx, cfg, webserver.Deps{}, webserver.Info{}, &notes)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "building the web server") {
		t.Errorf("the web server over an unreadable configuration returned %v, want it not built", err)
	}

	if notes.Len() != 0 {
		t.Errorf("the web server said %q, want nothing served", notes.String())
	}
}

// Once the interface or the web server starts it holds the terminal, where a
// token command that asks for a passphrase could not be answered, so the root
// command runs Jira's and the messaging service's before either starts.
func TestTheInterfaceAndTheWebServerStartWithTheTokenCommandsRun(t *testing.T) {
	t.Parallel()

	for name, args := range map[string][]string{"the interface": nil, "the web server": {"--web"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			dir := t.TempDir()
			record := filepath.Join(dir, "runs")
			command := filepath.Join(dir, "token")

			writeExecutable(t, command, "#!/bin/sh\nprintf 'x\\n' >> '"+record+"'\nprintf 'a-token\\n'\n", 0o700)

			writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net", "token_command": "`+command+`"}}`)

			runsAtStart := -1
			countRuns := func() {
				ran, _ := os.ReadFile(record)
				runsAtStart = strings.Count(string(ran), "x")
			}

			// Act
			_, _, err := executeRoot(t, place{dir: dir, home: dir},
				func(context.Context, tui.Model, io.Reader, io.Writer) (tui.Next, error) {
					countRuns()

					return tui.Next{}, nil
				},
				func(string) cli.RunWeb {
					return func(context.Context, config.Config, webserver.Deps, webserver.Info, io.Writer) error {
						countRuns()

						return nil
					}
				},
				args...)

			// Assert
			if err != nil || runsAtStart != 1 {
				t.Errorf("workflow %v = %v, and %s started with the token command run %d times; want once",
					args, err, name, runsAtStart)
			}
		})
	}
}
