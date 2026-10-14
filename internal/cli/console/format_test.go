// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package console_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/cli/console"
)

// statusConfiguration is the exit status of a configuration problem, which a
// failure carries as data in a structured format.
const statusConfiguration = 3

// requestOutline is one line of the request log, as wiring writes it.
const requestOutline = "2026-10-14T09:30:00Z jira  GET  /rest/api/2/myself 200 184ms\n"

// sayingAs is what notes said in opts, over a pipe, print for what says.
func sayingAs(opts console.Options, says func(console.Notes)) string {
	var stderr bytes.Buffer

	says(console.New(&stderr, opts))

	return stderr.String()
}

// entries are the JSON objects printed, one a line, failing the test when a
// line is not one.
func entries(t *testing.T, printed string) []map[string]any {
	t.Helper()

	var decoded []map[string]any

	for line := range strings.SplitSeq(strings.TrimSuffix(printed, "\n"), "\n") {
		var entry map[string]any

		err := json.Unmarshal([]byte(line), &entry)
		if err != nil {
			t.Fatalf("line %q is not one JSON object: %v", line, err)
		}

		decoded = append(decoded, entry)
	}

	return decoded
}

func TestQuietDropsNotesButKeepsWarningsAndErrors(t *testing.T) {
	t.Parallel()

	// Act
	printed := sayingAs(console.Options{Verbosity: console.Quiet}, func(notes console.Notes) {
		notes.Note("Created the branch.")
		_, _ = notes.Writer().Write([]byte("to #team\n"))
		notes.Warn("Posting untagged.")
		notes.Fail(errRefused, statusConfiguration)
	})

	// Assert
	if printed != "WARN Posting untagged.\nERROR "+errRefused.Error()+"\n" {
		t.Errorf("printed %q, want the warning and the error alone", printed)
	}
}

func TestVerboseSaysEachRequestOutlineAsADebugLine(t *testing.T) {
	t.Parallel()

	// Act
	printed := sayingAs(console.Options{Verbosity: console.Verbose}, func(notes console.Notes) {
		_, _ = notes.DebugWriter().Write([]byte(requestOutline))
	})

	// Assert
	if printed != "DEBUG jira GET /rest/api/2/myself 200 184ms\n" {
		t.Errorf("printed %q, want the outline after its tag, with no time of its own", printed)
	}
}

func TestNotesSayNoDebugLineUnlessVerbose(t *testing.T) {
	t.Parallel()

	// Act
	printed := sayingAs(console.Options{}, func(notes console.Notes) {
		_, _ = notes.DebugWriter().Write([]byte(requestOutline))
		notes.Debug("asked Jira")
	})

	// Assert
	if printed != "" {
		t.Errorf("printed %q, want nothing", printed)
	}
}

func TestJSONIsOneObjectALineWithTheExitStatusOnAnError(t *testing.T) {
	t.Parallel()

	// Act
	printed := sayingAs(console.Options{Format: console.JSON}, func(notes console.Notes) {
		notes.Note("No pull requests are waiting on your review.")
		notes.Fail(errRefused, statusConfiguration)
	})

	// Assert
	said := entries(t, printed)
	if len(said) != 2 {
		t.Fatalf("printed %q, want two objects", printed)
	}

	if said[0]["level"] != "info" || said[0]["msg"] != "No pull requests are waiting on your review." {
		t.Errorf("note = %v, want level info and the sentence", said[0])
	}

	if said[1]["level"] != "error" || said[1]["msg"] != errRefused.Error() || said[1]["exit_status"] != 3.0 {
		t.Errorf("error = %v, want level error, the message and exit_status 3", said[1])
	}

	for _, entry := range said {
		if stamp, isText := entry["time"].(string); !isText || !stamped(stamp) {
			t.Errorf("entry %v carries no RFC 3339 time", entry)
		}
	}
}

func TestLogfmtIsOneRecordALineWithTheExitStatusOnAnError(t *testing.T) {
	t.Parallel()

	// Act
	printed := sayingAs(console.Options{Format: console.Logfmt}, func(notes console.Notes) {
		notes.Warn("Posting untagged.")
		notes.Fail(errRefused, statusConfiguration)
	})

	// Assert
	lines := strings.Split(strings.TrimSuffix(printed, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("printed %q, want two records", printed)
	}

	if !strings.HasPrefix(lines[0], "time=") || !strings.HasSuffix(lines[0], ` level=warn msg="Posting untagged."`) {
		t.Errorf("warning = %q, want its time, level and message", lines[0])
	}

	if !strings.HasSuffix(lines[1], ` level=error msg="`+errRefused.Error()+`" exit_status=3`) {
		t.Errorf("error = %q, want its level, message and exit status", lines[1])
	}
}

// stamped reports whether stamp is an RFC 3339 time.
func stamped(stamp string) bool {
	_, err := time.Parse(time.RFC3339, stamp)

	return err == nil
}

func TestParseFormatNamesEachFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want console.Format
	}{
		{name: "text", want: console.Text},
		{name: "json", want: console.JSON},
		{name: "logfmt", want: console.Logfmt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Act
			format, err := console.ParseFormat(tt.name)

			// Assert
			if err != nil || format != tt.want || format.String() != tt.name {
				t.Errorf("ParseFormat(%q) = %v (%q), %v; want %v named as it was", tt.name, format, format, err, tt.want)
			}
		})
	}
}

func TestParseFormatRefusesAFormatItDoesNotKnow(t *testing.T) {
	t.Parallel()

	// Act
	_, err := console.ParseFormat("xml")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "text, json or logfmt") {
		t.Errorf("ParseFormat(xml) = %v, want a refusal naming the formats", err)
	}
}

func TestADebugLineKeepsAnOutlineWithNoTimeWhole(t *testing.T) {
	t.Parallel()

	// Act
	printed := sayingAs(console.Options{Verbosity: console.Verbose}, func(notes console.Notes) {
		_, _ = notes.DebugWriter().Write([]byte("forge POST /graphql 502 2s\n"))
	})

	// Assert
	if printed != "DEBUG forge POST /graphql 502 2s\n" {
		t.Errorf("printed %q, want every word of the outline", printed)
	}
}
