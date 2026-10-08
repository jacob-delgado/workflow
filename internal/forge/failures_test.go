// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

// A forge's failure at any step of a read or a write is reported as what it
// is — a token not accepted, a request refused, no API there, the forge in
// trouble — rather than read as nothing done, or as some other failure.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
)

// activityStart is the start of the day every activity read here asks about.
func activityStart() time.Time {
	return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
}

// statusErrors is what each failing status a forge answers is reported as.
func statusErrors() map[int]error {
	return map[int]error{
		http.StatusUnauthorized:        forge.ErrUnauthorized,
		http.StatusForbidden:           forge.ErrRefused,
		http.StatusNotFound:            forge.ErrNoAPI,
		http.StatusInternalServerError: forge.ErrUnexpectedStatus,
	}
}

func TestActivityReportsAForgesFailingAnswer(t *testing.T) {
	t.Parallel()

	for _, kind := range []forge.Kind{forge.KindGitHub, forge.KindGitLab} {
		for status, want := range statusErrors() {
			t.Run(kind.String()+" "+http.StatusText(status), func(t *testing.T) {
				t.Parallel()

				// Arrange
				client, _ := recordingForge(t, answering(status, `{"message":"no"}`))

				// Act
				activity, err := client.Activity(t.Context(), kind, activityStart(), activityStart().AddDate(0, 0, 1))

				// Assert
				if !errors.Is(err, want) || len(activity.Events) != 0 {
					t.Errorf("Activity = %+v, %v; want nothing read and %v", activity, err, want)
				}
			})
		}
	}
}

func TestActivityOnAForgeItCannotNameAsksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, answering(http.StatusOK, "[]"))

	// Act
	_, err := client.Activity(t.Context(), forge.KindUnknown, activityStart(), activityStart().AddDate(0, 0, 1))

	// Assert
	if !errors.Is(err, forge.ErrUnknownForge) || len(*seen) != 0 {
		t.Errorf("Activity = %v after %d requests, want ErrUnknownForge and none", err, len(*seen))
	}
}

func TestGitHubActivityReportsWhicheverReadFails(t *testing.T) {
	t.Parallel()

	reviewed := `{"total_count":1,"items":[{"number":3,"html_url":"https://github.com/o/r/pull/3",` +
		`"repository_url":"https://api.github.com/repos/o/r"}]}`

	// Each step's read is told apart by what it asks, and each case fails one.
	steps := map[string]func(recorded) bool{
		"the opened search": func(asked recorded) bool { return strings.Contains(asked.query, "created%3A") },
		"the merged search": func(asked recorded) bool { return strings.Contains(asked.query, "is%3Amerged") },
		"who the token is":  func(asked recorded) bool { return asked.path == userPath },
		"the reviewed search": func(asked recorded) bool {
			return strings.Contains(asked.query, "reviewed-by%3A")
		},
		"a pull's reviews": func(asked recorded) bool { return strings.HasSuffix(asked.path, "/reviews") },
	}

	for name, failing := range steps {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, func(asked recorded) (int, string) {
				switch {
				case failing(asked):
					return http.StatusInternalServerError, ""
				case asked.path == userPath:
					return http.StatusOK, githubAna
				case strings.Contains(asked.query, "reviewed-by%3A"):
					return http.StatusOK, reviewed
				case strings.HasSuffix(asked.path, "/reviews"):
					return http.StatusOK, "[]"
				default:
					return http.StatusOK, githubFoundNothing
				}
			})

			// Act
			_, err := client.Activity(t.Context(), forge.KindGitHub, activityStart(), activityStart().AddDate(0, 0, 1))

			// Assert
			if !errors.Is(err, forge.ErrUnexpectedStatus) {
				t.Errorf("Activity = %v, want the failed read's ErrUnexpectedStatus", err)
			}
		})
	}
}

func TestJobLogReportsAForgesFailingAnswer(t *testing.T) {
	t.Parallel()

	for status, want := range statusErrors() {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := recordingForge(t, answering(status, `{"message":"no"}`))

			// Act
			log, err := client.JobLog(t.Context(), gitlabRepo(), forge.Check{ID: "501", LogAvailable: true})

			// Assert
			if !errors.Is(err, want) || log != (forge.JobLog{}) {
				t.Errorf("JobLog = %+v, %v; want nothing read and %v", log, err, want)
			}
		})
	}
}

func TestJobLogReportsAForgeItCannotReach(t *testing.T) {
	t.Parallel()

	gone := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	gone.Close()

	cases := map[string]string{
		"an address that cannot be used": unusableBase,
		"a forge that does not answer":   gone.URL,
	}

	for name, base := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forge.New(httpx.Client(time.Second).Do, base, secret)

			// Act
			_, err := client.JobLog(t.Context(), gitlabRepo(), forge.Check{ID: "501", LogAvailable: true})

			// Assert
			if !errors.Is(err, forge.ErrUnreachable) {
				t.Errorf("JobLog = %v, want ErrUnreachable", err)
			}
		})
	}
}

func TestJobLogAsksNothingItCannotAsk(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		repo  forge.Repo
		check forge.Check
		token forge.Token
		want  error
	}{
		"a check with no id": {
			repo: gitlabRepo(), check: forge.Check{LogAvailable: true}, token: secret, want: forge.ErrNoLog,
		},
		"without a token": {
			repo: gitlabRepo(), check: forge.Check{ID: "501", LogAvailable: true}, token: "", want: forge.ErrNoToken,
		},
		"a forge it cannot name": {
			repo: unknownForge(), check: forge.Check{ID: "501", LogAvailable: true}, token: secret,
			want: forge.ErrUnknownForge,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forge.New(http.DefaultClient.Do, "https://forge.invalid", tt.token)

			// Act
			_, err := client.JobLog(t.Context(), tt.repo, tt.check)

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("JobLog = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestAWriteWithNoTokenAsksNothing(t *testing.T) {
	t.Parallel()

	pull := forge.PullRequest{Number: 42}

	cases := map[string]func(forge.Client) error{
		"a merge": func(client forge.Client) error {
			return client.Merge(t.Context(), githubRepo(), pull, forge.MergeCommit)
		},
		"a rerun": func(client forge.Client) error {
			_, err := client.RerunChecks(t.Context(), githubRepo(), pull, "abc123")

			return err
		},
	}

	for name, write := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client := forge.New(http.DefaultClient.Do, "https://forge.invalid", "")

			// Act
			err := write(client)

			// Assert
			if !errors.Is(err, forge.ErrNoToken) {
				t.Errorf("the write = %v, want ErrNoToken before anything is sent", err)
			}
		})
	}
}

func TestReviewersTurnedDownInTroubleAreAskedForOneByOne(t *testing.T) {
	t.Parallel()

	// Arrange
	// GitHub fails the call naming both reviewers; asked alone, each is added.
	client, seen := recordingForge(t, func(asked recorded) (int, string) {
		switch {
		case asked.path == githubPullsPath:
			return http.StatusCreated, githubPull43
		case asked.path == githubReviewersPath && len(names(asked.body, "reviewers")) > 1:
			return http.StatusInternalServerError, ""
		default:
			return http.StatusCreated, "{}"
		}
	})

	// Act
	_, err := client.CreatePullRequest(t.Context(), githubRepo(), forge.NewPullRequest{
		Title: prTitle, Head: featureBranch, Base: baseBranch, Reviewers: []string{userAna, userBen},
	})

	// Assert
	if err != nil || requestsTo(*seen, githubReviewersPath) != 3 {
		t.Errorf("CreatePullRequest = %v after %d reviewer requests; want both asked again alone",
			err, requestsTo(*seen, githubReviewersPath))
	}
}
