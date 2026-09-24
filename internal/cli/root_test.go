// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

// runRoot runs the root command in dir over a stand-in interface and web
// server. Like run, it sets the working directory and the environment, so its
// tests are not parallel.
func runRoot(t *testing.T, dir string, args ...string) *rootRun {
	t.Helper()

	for name, value := range isolatedEnvironment(t.TempDir()) {
		t.Setenv(name, value)
	}

	t.Chdir(dir)

	var (
		ran            rootRun
		stdout, stderr bytes.Buffer
	)

	root := cli.NewRootCmdOver(unusedPrompt(t), ran.runInterface, ran.serveWeb)
	root.SetArgs(args)
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	ran.err = root.ExecuteContext(t.Context())
	ran.stdout = stdout.String()
	ran.stderr = stderr.String()

	return &ran
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
