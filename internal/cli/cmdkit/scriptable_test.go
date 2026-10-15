// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cmdkit_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jacob-delgado/workflow/internal/cli/cmdkit"
	"github.com/jacob-delgado/workflow/internal/cli/console"
)

// errPipeGone is a stdout whose reader has gone, as when a pipe's far end
// exits early.
var errPipeGone = errors.New("the pipe's reader has gone")

// errOutcome is what a command's work ended with, whatever became of its
// output.
var errOutcome = errors.New("the command's own failure")

// refusing fails every write, counting each one it is asked to make.
type refusing struct {
	asked *int
}

var _ io.Writer = refusing{}

func (r refusing) Write([]byte) (int, error) {
	*r.asked++

	return 0, errPipeGone
}

func TestCheckedOutputRefusesEveryWriteAfterOneFails(t *testing.T) {
	t.Parallel()

	// Arrange
	asked := 0
	checked := &cmdkit.CheckedOutput{Out: refusing{asked: &asked}}

	// Act: the first write fails
	_, first := checked.Write([]byte("first"))

	// Assert: as the stdout's own failure, and the output's
	if !errors.Is(first, errPipeGone) || !errors.Is(first, cmdkit.ErrOutputUnwritten) {
		t.Errorf("the first write = %v, want it to say the output went nowhere, and why", first)
	}

	// Act: a write after it
	written, second := checked.Write([]byte("second"))

	// Assert: is refused without asking the stdout again
	if written != 0 || !errors.Is(second, cmdkit.ErrOutputUnwritten) || asked != 1 {
		t.Errorf("the write after = %d, %v, after %d writes asked; want none made, refused as the first", written,
			second, asked)
	}
}

func TestCheckedOutputPassesAWriteThatLands(t *testing.T) {
	t.Parallel()

	// Arrange
	var stdout bytes.Buffer

	checked := &cmdkit.CheckedOutput{Out: &stdout}

	// Act
	written, err := checked.Write([]byte("the report\n"))

	// Assert
	if written != len("the report\n") || err != nil || stdout.String() != "the report\n" ||
		checked.Failed(nil) != nil || !errors.Is(checked.Failed(errOutcome), errOutcome) {
		t.Errorf("write = %d, %v, printed %q; want it written, and the outcome left as it was", written, err,
			stdout.String())
	}
}

func TestFailedSaysTheOutputWentNowhereOnce(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		outcome  error
		joinsOwn bool
	}{
		"a run that succeeded":              {outcome: nil, joinsOwn: false},
		"a run that failed on its own":      {outcome: errOutcome, joinsOwn: true},
		"a run that says the output failed": {outcome: fmt.Errorf("printing: %w", cmdkit.ErrOutputUnwritten)},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			asked := 0
			checked := &cmdkit.CheckedOutput{Out: refusing{asked: &asked}}
			_, _ = checked.Write([]byte("the report"))

			// Act
			err := checked.Failed(tt.outcome)

			// Assert
			if !errors.Is(err, cmdkit.ErrOutputUnwritten) || errors.Is(err, errOutcome) != tt.joinsOwn {
				t.Errorf("%s = %v; want the output said to have gone nowhere, the run's own failure kept: %v", name,
					err, tt.joinsOwn)
			}

			if strings.Count(err.Error(), cmdkit.ErrOutputUnwritten.Error()) != 1 {
				t.Errorf("%s = %q, want the output's failure said once", name, err)
			}
		})
	}
}

func TestTheCheckedStdoutIsTheFileItIsOver(t *testing.T) {
	t.Parallel()

	// Arrange
	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("making a file for stdout: %v", err)
	}

	t.Cleanup(func() { _ = file.Close() })

	checked := &cmdkit.CheckedOutput{Out: file}

	// Act
	stream := checked.Stream()

	// Assert
	asFile, isFile := stream.(interface {
		io.ReadWriteCloser
		Fd() uintptr
	})
	if !isFile || asFile.Fd() != file.Fd() {
		t.Fatalf("the stream over a file = %T, want one with the file's descriptor", stream)
	}

	_, writeErr := fmt.Fprint(asFile, "the report")
	_, readErr := asFile.Read(make([]byte, 1))

	if writeErr != nil || !errors.Is(readErr, io.EOF) || asFile.Close() != nil {
		t.Errorf("write %v, read %v; want it written, read and closed as the file is", writeErr, readErr)
	}
}

func TestTheCheckedStdoutOverAnyOtherStreamIsTheCheckAlone(t *testing.T) {
	t.Parallel()

	// Arrange
	var stdout bytes.Buffer

	checked := &cmdkit.CheckedOutput{Out: &stdout}

	// Act
	stream := checked.Stream()

	// Assert
	if stream != io.Writer(checked) {
		t.Errorf("the stream over a buffer = %T, want the check itself", stream)
	}
}

func TestTerminalOutIsTheStreamBeneathTheCheck(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer

	file, err := os.Create(filepath.Join(t.TempDir(), "stdout"))
	if err != nil {
		t.Fatalf("making a file for stdout: %v", err)
	}

	t.Cleanup(func() { _ = file.Close() })

	cases := map[string]struct {
		out  io.Writer
		want io.Writer
	}{
		"checked":         {out: (&cmdkit.CheckedOutput{Out: &stdout}).Stream(), want: &stdout},
		"checked, a file": {out: (&cmdkit.CheckedOutput{Out: file}).Stream(), want: file},
		"handed as it is": {out: &stdout, want: &stdout},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			cmd := &cobra.Command{}
			cmd.SetOut(tt.out)

			// Act
			got := cmdkit.TerminalOut(cmd)

			// Assert
			if got != tt.want {
				t.Errorf("TerminalOut over %s = %T, want the stream beneath", name, got)
			}
		})
	}
}

func TestAsideSaysTheArtifactAsANote(t *testing.T) {
	t.Parallel()

	// Arrange
	var stdout, stderr bytes.Buffer

	out := cmdkit.Output{Artifact: &stdout, Notes: console.New(&stderr, console.Options{})}

	// Act
	fmt.Fprint(out.Aside().Artifact, "the preview")

	// Assert
	if stdout.String() != "" || !strings.Contains(stderr.String(), "the preview") {
		t.Errorf("stdout %q, stderr %q; want the preview said beside the artifact, on stderr", stdout.String(),
			stderr.String())
	}
}

// writeSaid is what a write tested here says around its confirmation.
func writeSaid() cmdkit.WritePrompt {
	return cmdkit.WritePrompt{Question: "Open it?", DryRun: "dry run: would open it", Declined: "Not opened."}
}

func TestProceedGoesAheadOnlyWhenTheWriteIsMeantToGo(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		opts    cmdkit.WriteOptions
		answer  bool
		readErr error
		goes    bool
		asked   bool
		said    string
		wantErr error
	}{
		"a dry run":          {opts: cmdkit.WriteOptions{DryRun: true}, said: "dry run: would open it"},
		"--yes":              {opts: cmdkit.WriteOptions{Yes: true}, goes: true},
		"a yes":              {answer: true, goes: true, asked: true},
		"a no":               {answer: false, asked: true, said: "Not opened."},
		"nothing to answer":  {readErr: cmdkit.ErrNoTerminal, asked: true, wantErr: cmdkit.ErrNoTerminal},
		"an answer not read": {readErr: errKeyboardGone, asked: true, wantErr: errKeyboardGone},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			var stderr bytes.Buffer

			asked := false
			confirm := func(string) (bool, error) {
				asked = true

				return tt.answer, tt.readErr
			}

			// Act
			goes, err := tt.opts.Proceed(console.New(&stderr, console.Options{}), confirm, writeSaid())

			// Assert
			if goes != tt.goes || asked != tt.asked || !errors.Is(err, tt.wantErr) || (tt.wantErr == nil && err != nil) {
				t.Errorf("%s = %v, %v after asking: %v; want %v, %v after asking: %v", name, goes, err, asked, tt.goes,
					tt.wantErr, tt.asked)
			}

			if tt.said != "" && !strings.Contains(stderr.String(), tt.said) {
				t.Errorf("%s said %q, want %q", name, stderr.String(), tt.said)
			}
		})
	}
}

func TestProceedWithNothingToAnswerSaysToPassYes(t *testing.T) {
	t.Parallel()

	// Arrange
	var opts cmdkit.WriteOptions

	nothing := func(string) (bool, error) { return false, cmdkit.ErrNoTerminal }

	// Act
	_, err := opts.Proceed(console.New(io.Discard, console.Options{}), nothing, writeSaid())

	// Assert
	if err == nil || !strings.HasSuffix(err.Error(), "; pass --yes to go ahead without asking") {
		t.Errorf("proceed with no terminal = %v, want it to say to pass --yes", err)
	}
}

func TestAScriptableWriteReadsTheRootsDryRunAndItsOwnYes(t *testing.T) {
	t.Parallel()

	// Arrange
	env, _ := inATempDir(t)

	var opts cmdkit.WriteOptions

	root := probeTree(env, func(*cobra.Command, []string) error { return nil })
	write := &cobra.Command{Use: "write", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return nil }}
	opts.AddFlags(write, cmdkit.ConfirmationHelp)
	root.AddCommand(write)

	// Act
	_, err := runTree(t, root, "-n", "write", "-y")

	// Assert
	if err != nil || !opts.DryRun || !opts.Yes || write.Flag("yes").Usage != cmdkit.ConfirmationHelp {
		t.Errorf("write -n -y = %v, dry run %v, yes %v; want both read", err, opts.DryRun, opts.Yes)
	}
}

func TestEncodeJSONIndentsWhatItWrites(t *testing.T) {
	t.Parallel()

	// Arrange
	var out bytes.Buffer

	// Act
	err := cmdkit.EncodeJSON(&out, map[string]int{"open": 2})

	// Assert
	if err != nil || out.String() != "{\n  \"open\": 2\n}\n" {
		t.Errorf("encode = %v, wrote %q; want it indented two spaces", err, out.String())
	}
}

func TestEncodeJSONSaysWhenTheOutputCannotBeEncoded(t *testing.T) {
	t.Parallel()

	// Act
	err := cmdkit.EncodeJSON(io.Discard, map[string]any{"fn": func() {}})

	// Assert
	if err == nil || !strings.HasPrefix(err.Error(), "encoding the output: ") {
		t.Errorf("encode of a function = %v, want it refused, saying the output was not encoded", err)
	}
}

func TestIsAnyFindsATargetErrWraps(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		err   error
		found bool
	}{
		"a wrapped target": {err: fmt.Errorf("posting: %w", errPipeGone), found: true},
		"no target":        {err: errOutcome, found: false},
		"no error":         {err: nil, found: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			found := cmdkit.IsAny(tt.err, cmdkit.ErrUsage, errPipeGone)

			// Assert
			if found != tt.found {
				t.Errorf("IsAny(%v) = %v, want %v", tt.err, found, tt.found)
			}
		})
	}
}
