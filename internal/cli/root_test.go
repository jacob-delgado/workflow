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
	"github.com/jacob-delgado/workflow/internal/store"
	"github.com/jacob-delgado/workflow/internal/tui"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// What the stand-in interface and web server write to the stream each is
// handed, so a test can tell which stream that was.
const (
	interfaceWrote = "interface"
	serverWrote    = "server"
)

// rootRun is one run of the root command over a stand-in interface and web
// server: how often it started each and what it handed them, what it said on
// stdout and stderr, and how it ended.
type rootRun struct {
	interfaces int
	model      tui.Model
	servers    int
	cfg        config.Config
	info       webserver.Info
	stdout     string
	stderr     string
	err        error
}

// runInterface stands in for tui.Run, keeping the model it was handed and
// writing interfaceWrote where the interface draws.
func (r *rootRun) runInterface(_ context.Context, model tui.Model, out io.Writer) error {
	r.interfaces++
	r.model = model

	fmt.Fprint(out, interfaceWrote)

	return nil
}

// serveWeb stands in for the web server, keeping the configuration and the
// facts about the run it was handed and writing serverWrote to its notes.
func (r *rootRun) serveWeb(
	_ context.Context, cfg config.Config, _ webserver.Deps, info webserver.Info, notes io.Writer,
) error {
	r.servers++
	r.cfg = cfg
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
// server. Like run, it sets the working directory and the environment, so its
// tests are not parallel.
func runRoot(t *testing.T, dir string, args ...string) *rootRun {
	t.Helper()

	var ran rootRun

	ran.stdout, ran.stderr, ran.err = executeRoot(t, dir, ran.runInterface, ran.serveWeb, args...)

	return &ran
}

// executeRoot runs the root command in dir over the interface and web server
// given, and returns what it said on stdout and on stderr, and how it ended.
func executeRoot(
	t *testing.T, dir string, run cli.RunInterface, serve cli.RunWeb, args ...string,
) (string, string, error) {
	t.Helper()

	for name, value := range isolatedEnvironment(t.TempDir()) {
		t.Setenv(name, value)
	}

	t.Chdir(dir)

	var stdout, stderr bytes.Buffer

	root := cli.NewRootCmdOver(unusedPrompt(t), run, serve)
	root.SetArgs(args)
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.ExecuteContext(t.Context())

	return stdout.String(), stderr.String(), err
}

func TestBareWorkflowOpensTheInterfaceWithItsWritesLive(t *testing.T) {
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

func TestDryRunOpensTheInterfaceHoldingItsWritesBack(t *testing.T) {
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

// storeKept is where the run's own environment keeps the store, and whether
// its directory exists there.
func storeKept(t *testing.T) (string, bool) {
	t.Helper()

	dir, err := store.DefaultDir()
	if err != nil {
		t.Fatalf("finding the store directory: %v", err)
	}

	_, err = os.Stat(dir)

	return dir, err == nil
}

func TestADryRunOpensTheInterfaceWithNoStore(t *testing.T) {
	// Arrange
	// With a Jira to key it by, the interface seeds its issue list from the
	// store, and the store makes its directory even to read.
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net"}}`)

	// Act
	ran := runRoot(t, dir, "--dry-run")

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow --dry-run = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if kept, found := storeKept(t); found {
		t.Errorf("workflow --dry-run made the store at %s, want nothing on disk", kept)
	}
}

// The twin of the test above, so its Assert is seen to fail when the store is
// opened: the same interface, its writes live, opens the store to seed its list.
func TestTheInterfaceOpensTheStoreToSeedItsIssueList(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net"}}`)

	// Act
	ran := runRoot(t, dir)

	// Assert
	if ran.err != nil || ran.interfaces != 1 {
		t.Fatalf("workflow = %v, opened %d interfaces; want the interface", ran.err, ran.interfaces)
	}

	if kept, found := storeKept(t); !found {
		t.Errorf("workflow opened no store at %s, want its issue list seeded from it", kept)
	}
}

func TestColorTurnedOffOpensTheInterfaceWithoutHues(t *testing.T) {
	cases := map[string]struct {
		noColor       string
		configuration string
	}{
		"NO_COLOR set":   {noColor: "1", configuration: `{}`},
		"ui.color never": {noColor: "", configuration: `{"ui": {"color": "never"}}`},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			writeFile(t, dir, tt.configuration)
			t.Setenv("NO_COLOR", tt.noColor)

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
	// Arrange
	t.Setenv("NO_COLOR", "")

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

func TestTheWebFlagServesTheLoadedConfigurationInsteadOfTheInterface(t *testing.T) {
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
	// Act
	ran := runRoot(t, t.TempDir(), "--web", "--dry-run")

	// Assert
	if ran.err != nil || ran.servers != 1 || !ran.info.DryRun {
		t.Errorf("workflow --web --dry-run = %v, served %d times, told %+v; want the server told to hold writes back",
			ran.err, ran.servers, ran.info)
	}
}

func TestTheWebFlagSaysWhyTheConfigurationDidNotLoadAndServesAnyway(t *testing.T) {
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

func TestTheWebServerSaysWhereItServesAndStopsWithItsRun(t *testing.T) {
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

	if !strings.Contains(notes.String(), "serving http://"+addr) {
		t.Errorf("the web server said %q, want where it serves", notes.String())
	}
}

func TestTheWebServerRefusesAConfigurationPathItCannotRead(t *testing.T) {
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
	for name, args := range map[string][]string{"the interface": nil, "the web server": {"--web"}} {
		t.Run(name, func(t *testing.T) {
			// Arrange
			dir := t.TempDir()
			record := filepath.Join(dir, "runs")
			command := filepath.Join(dir, "token")

			err := os.WriteFile(command, []byte("#!/bin/sh\nprintf 'x\\n' >> '"+record+"'\nprintf 'a-token\\n'\n"), 0o700)
			if err != nil {
				t.Fatal(err)
			}

			writeFile(t, dir, `{"jira": {"base_url": "https://jira.example.net", "token_command": "`+command+`"}, `+
				`"messaging": {"token_command": "`+command+`", "channel": "#dev"}}`)

			runsAtStart := -1
			countRuns := func() {
				ran, _ := os.ReadFile(record)
				runsAtStart = strings.Count(string(ran), "x")
			}

			// Act
			_, _, err = executeRoot(t, dir,
				func(context.Context, tui.Model, io.Writer) error {
					countRuns()

					return nil
				},
				func(context.Context, config.Config, webserver.Deps, webserver.Info, io.Writer) error {
					countRuns()

					return nil
				},
				args...)

			// Assert
			if err != nil || runsAtStart != 2 {
				t.Errorf("workflow %v = %v, and %s started with the token commands run %d times; want once each",
					args, err, name, runsAtStart)
			}
		})
	}
}
