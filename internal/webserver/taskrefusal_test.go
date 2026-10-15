// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/taskwarrior"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
)

func TestAWriteTaskwarriorRefusesIs422WithItsWords(t *testing.T) {
	t.Parallel()

	// Taskwarrior's words say what in the line it could not take; they keep
	// all but where its data is and which sync server a hook named.
	notADate := "'xyz' is not a valid date in the 'Y-M-D' format."
	refused := "Taskwarrior refused the command"
	cases := map[string]struct {
		words, wantDetail string
		// homeUnknown has no home directory known.
		homeUnknown bool
	}{
		"its words": {words: notADate, wantDetail: refused + ": " + notADate},
		"its words, with no home known": {
			words: notADate, homeUnknown: true, wantDetail: refused + ": " + notADate,
		},
		"its data directory": {
			words:      "Could not open " + webtest.DataPath + "/taskchampion.sqlite3.",
			wantDetail: refused + ": Could not open the data directory/taskchampion.sqlite3.",
		},
		"your home": {
			words:      "Hook " + webtest.HomePath + "/.task/hooks/on-modify.check failed.",
			wantDetail: refused + ": Hook ~/.task/hooks/on-modify.check failed.",
		},
		"a line naming a URL": {
			words: "could not sync with https://" + syncServer + "/v1\n" + notADate, wantDetail: refused + ": " + notADate,
		},
		"a line naming a host and port": {
			words: notADate + "\nconnecting to " + syncServer + ":8080 failed", wantDetail: refused + ": " + notADate,
		},
		"a line naming an IPv4 address and port": {
			words: notADate + "\nretrying 192.168.1.10:10222", wantDetail: refused + ": " + notADate,
		},
		"a line naming an IPv6 address and port": {
			words: notADate + "\nretrying [fd00::17]:10222", wantDetail: refused + ": " + notADate,
		},
		"words on both sides of a line naming an address": {
			words:      "Hook on-modify.check failed.\nconnecting to " + syncServer + ":8080 failed\n" + notADate,
			wantDetail: refused + ": Hook on-modify.check failed.\n" + notADate,
		},
		"a clock time": {
			words: "The due date is 10:30 today.", wantDetail: refused + ": The due date is 10:30 today.",
		},
		"a date and time": {
			words:      "'2026-02-30T08:00' is not a valid date in the 'Y-M-D' format.",
			wantDetail: refused + ": '2026-02-30T08:00' is not a valid date in the 'Y-M-D' format.",
		},
		"a date and time in a month past twelve": {
			words:      "'2026-13-02T10:30' is not a valid date in the 'Y-M-D' format.",
			wantDetail: refused + ": '2026-13-02T10:30' is not a valid date in the 'Y-M-D' format.",
		},
		"a named date and time": {
			words:      "'tomorrowT10:00' is not a valid date in the 'Y-M-D' format.",
			wantDetail: refused + ": 'tomorrowT10:00' is not a valid date in the 'Y-M-D' format.",
		},
		"a line naming a host and port beside a date and time": {
			words:      notADate + "\nat 2026-02-30T08:00 connecting to " + syncServer + ":8080 failed",
			wantDetail: refused + ": " + notADate,
		},
		// Run into a host's name, a date and time is read as part of the host
		// and its port, not taken out as a time before the address is looked for.
		"a line naming a host run into a date and time": {
			words:      notADate + "\nretrying " + syncServer + "2026-02-30T08:00",
			wantDetail: refused + ": " + notADate,
		},
		"a taskrc line it could not read": {
			words:      "Malformed entry 'sync.encryption_secret hunter2' in config file.\n" + notADate,
			wantDetail: refused + ": " + notADate,
		},
		"nothing but addresses": {words: "https://" + syncServer + "\n" + syncServer + ":8080", wantDetail: refused},
		"no words at all":       {words: "", wantDetail: refused},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()
			fake.Fail["modify"] = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, tt.words)

			deps := tasksDeps(fake)
			if tt.homeUnknown {
				deps.Repositories.Home = ""
			}

			// Act
			recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()),
				http.MethodPost, taskPath(webtest.StartedUUID, modifyWrite), `{"line":"due:xyz"}`)

			// Assert
			failure := webtest.Decode[api.Problem](t, recorder)
			if recorder.Code != http.StatusUnprocessableEntity || failure.Detail != tt.wantDetail {
				t.Errorf("answer = %d %+v, want 422 with detail %q", recorder.Code, failure, tt.wantDetail)
			}
		})
	}
}

func TestARefusalKeepsWordsThatOnlyLookLikeAPlace(t *testing.T) {
	t.Parallel()

	// A data directory that is not an absolute path, and a home at the root,
	// name no place a word could be mistaken for: replacing them would rewrite
	// every "task" and every path.
	cases := map[string]struct {
		dataDir, home, words string
	}{
		"a relative data.location": {dataDir: "task", home: webtest.HomePath, words: "Cannot modify a deleted task."},
		"a home at the root":       {dataDir: webtest.DataPath, home: "/", words: "Could not read /etc/taskrc."},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			fake := webtest.FakeTaskwarrior()
			fake.Install.DataDir = tt.dataDir
			fake.Fail["modify"] = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, tt.words)

			deps := tasksDeps(fake)
			deps.Repositories.Home = tt.home

			// Act
			recorder := webtest.Send(t, webtest.Serve(t, deps, config.Default()),
				http.MethodPost, taskPath(webtest.StartedUUID, modifyWrite), `{"line":"due:xyz"}`)

			// Assert
			want := "Taskwarrior refused the command: " + tt.words
			if failure := webtest.Decode[api.Problem](t, recorder); failure.Detail != want {
				t.Errorf("detail = %q, want %q", failure.Detail, want)
			}
		})
	}
}

func TestARefusalNamesYourHomeWhenTaskwarriorCannotSayWhereItsDataIs(t *testing.T) {
	t.Parallel()

	// Arrange
	// Asked where its data is, Taskwarrior does not answer, so the path its
	// refusal names is written from your home rather than as the data
	// directory.
	fake := webtest.FakeTaskwarrior()
	fake.Fail["install"] = taskwarrior.ErrNotInstalled
	words := "Could not open " + webtest.DataPath + "/taskchampion.sqlite3."
	fake.Fail["modify"] = fmt.Errorf("%w: %s", taskwarrior.ErrRefused, words)

	// Act
	recorder := webtest.Send(t, webtest.Serve(t, tasksDeps(fake), config.Default()),
		http.MethodPost, taskPath(webtest.StartedUUID, modifyWrite), `{"line":"due:xyz"}`)

	// Assert
	want := "Taskwarrior refused the command: Could not open ~/.local/share/task/taskchampion.sqlite3."
	if failure := webtest.Decode[api.Problem](t, recorder); recorder.Code != http.StatusUnprocessableEntity ||
		failure.Detail != want {
		t.Errorf("answer = %d %+v, want 422 with detail %q", recorder.Code, failure, want)
	}
}
