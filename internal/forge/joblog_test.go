// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/forge"
	"github.com/jacob-delgado/workflow/internal/httpx"
)

// failedRun is a GitHub Actions check run whose log can be read.
func failedRun() forge.Check {
	return forge.Check{ID: "901", Name: "race", State: forge.CIFailed, LogAvailable: true}
}

// heard is what a stand-in server was asked: each path, and the Authorization
// it carried.
type heard struct {
	lock  sync.Mutex
	paths []string
	auths []string
}

func (h *heard) note(request *http.Request) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.paths = append(h.paths, request.URL.Path)
	h.auths = append(h.auths, request.Header.Get("Authorization"))
}

// logServers are GitHub's API, which redirects a job's log to the address
// redirect gives it, and the storage it redirects to, which serves log; each
// over TLS, as both are in earnest. The client refuses redirects as workflow's
// does, and trusts both servers' certificate.
func logServers(t *testing.T, log string, redirect func(blob string) string) (forge.Client, *heard, *heard) {
	t.Helper()

	return logServersAnswering(t, func(writer http.ResponseWriter, request *http.Request, blob string) {
		http.Redirect(writer, request, redirect(blob), http.StatusFound)
	}, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		_, _ = writer.Write([]byte(log))
	})
}

// logServersAnswering is logServers with GitHub's answer and the storage's
// given whole: GitHub's is told the storage's address.
func logServersAnswering(
	t *testing.T, github func(http.ResponseWriter, *http.Request, string), storage http.HandlerFunc,
) (forge.Client, *heard, *heard) {
	t.Helper()

	api, stored := &heard{}, &heard{}

	blob := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		stored.note(request)
		storage(writer, request)
	}))
	t.Cleanup(blob.Close)

	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		api.note(request)
		github(writer, request, blob.URL)
	}))
	t.Cleanup(server.Close)

	client := httpx.Client(10 * time.Second)
	client.Transport = server.Client().Transport

	return forge.New(client.Do, server.URL, secret), api, stored
}

func TestJobLogFollowsGitHubsRedirectWithoutTheToken(t *testing.T) {
	t.Parallel()

	// Arrange
	client, api, storage := logServers(t, "--- FAIL: TestRetry\nFAIL\n", func(blob string) string { return blob + "/log" })

	// Act
	log, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if err != nil || log.Text != "--- FAIL: TestRetry\nFAIL" || log.Truncated {
		t.Fatalf("JobLog = %+v, %v; want the log as written", log, err)
	}

	if len(api.paths) != 1 || api.paths[0] != "/repos/example/repo/actions/jobs/901/logs" || api.auths[0] == "" {
		t.Errorf("GitHub was asked %v with %v; want the job's logs, with the token", api.paths, api.auths)
	}

	if len(storage.auths) != 1 || storage.auths[0] != "" {
		t.Errorf("the storage was sent Authorization %q; want none", storage.auths)
	}
}

func TestJobLogRefusesARedirectToPlainHTTP(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _, storage := logServers(t, "log", func(blob string) string {
		return strings.Replace(blob, "https://", "http://", 1) + "/log"
	})

	// Act
	_, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if !errors.Is(err, forge.ErrInsecureLog) || len(storage.paths) != 0 {
		t.Errorf("JobLog = %v, storage asked %v; want it refused before anything is sent", err, storage.paths)
	}
}

func TestJobLogSaysWhenGitHubRedirectsNowhere(t *testing.T) {
	t.Parallel()

	// Arrange
	client, _, storage := logServersAnswering(t, func(writer http.ResponseWriter, _ *http.Request, _ string) {
		writer.WriteHeader(http.StatusFound)
	}, func(http.ResponseWriter, *http.Request) {})

	// Act
	_, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if !errors.Is(err, forge.ErrLogNotRedirected) || errors.Is(err, forge.ErrInsecureLog) || len(storage.paths) != 0 {
		t.Errorf("JobLog = %v, storage asked %v; want ErrLogNotRedirected and nothing sent", err, storage.paths)
	}
}

func TestJobLogTellsTheStoragesRefusalFromTheForges(t *testing.T) {
	t.Parallel()

	cases := map[string]int{"an expired address": http.StatusForbidden, "a log gone": http.StatusNotFound}

	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _, _ := logServersAnswering(t, func(writer http.ResponseWriter, request *http.Request, blob string) {
				http.Redirect(writer, request, blob+"/log", http.StatusFound)
			}, func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(status) })

			// Act
			_, err := client.JobLog(t.Context(), githubRepo(), failedRun())

			// Assert
			_, advised := forge.Advice(err)
			if !errors.Is(err, forge.ErrLogStorage) || errors.Is(err, forge.ErrNoAPI) || advised {
				t.Errorf("JobLog = %v (advice %v); want ErrLogStorage, not the forge's refusal", err, advised)
			}
		})
	}
}

func TestJobLogKeepsTheLastLinesOfALongLog(t *testing.T) {
	t.Parallel()

	// Arrange
	var long strings.Builder
	for line := range 1000 {
		long.WriteString("line " + strconv.Itoa(line) + "\n")
	}

	client, _, _ := logServers(t, long.String(), func(blob string) string { return blob + "/log" })

	// Act
	log, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	lines := strings.Split(log.Text, "\n")
	if err != nil || !log.Truncated || len(lines) != 400 || lines[len(lines)-1] != "line 999" {
		t.Errorf("JobLog kept %d lines ending %q, truncated %v, %v; want the last 400", len(lines),
			lines[len(lines)-1], log.Truncated, err)
	}
}

func TestJobLogKeepsTheRealEndOfALogPastAnyReadLimit(t *testing.T) {
	t.Parallel()

	// Arrange
	huge := strings.Repeat("filler line\n", (33<<20)/len("filler line\n")) + "the real end\n"
	client, _, _ := logServers(t, huge, func(blob string) string { return blob + "/log" })

	// Act
	log, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if err != nil || !log.Truncated || !strings.HasSuffix(log.Text, "\nthe real end") {
		t.Errorf("JobLog ends %q, truncated %v, %v; want the log's last line", log.Text[max(0, len(log.Text)-40):],
			log.Truncated, err)
	}
}

func TestJobLogNeutralizesWhatWouldDriveTheTerminal(t *testing.T) {
	t.Parallel()

	// Arrange
	colored := "\x1b[31mFAIL\x1b[0m\x1b]0;owned\x07\n"
	client, _, _ := logServers(t, colored, func(blob string) string { return blob + "/log" })

	// Act
	log, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if err != nil || log.Text != "FAIL" {
		t.Errorf("JobLog = %q, %v; want the colors and the title escape gone", log.Text, err)
	}
}

func TestJobLogReadsAGitLabJobsTrace(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, routing(map[string]string{"/projects/group%2Fsub%2Frepo/jobs/501/trace": "boom\n"}))

	// Act
	log, err := client.JobLog(t.Context(), gitlabRepo(), forge.Check{ID: "501", LogAvailable: true})

	// Assert
	if err != nil || log.Text != "boom" || len(*seen) != 1 {
		t.Errorf("JobLog = %+v, %v after %v; want the trace", log, err, *seen)
	}
}

func TestJobLogOfACheckWithNoLogAsksNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := recordingForge(t, routing(map[string]string{}))

	// Act
	_, err := client.JobLog(t.Context(), githubRepo(), forge.Check{Name: "lint"})

	// Assert
	if !errors.Is(err, forge.ErrNoLog) || len(*seen) != 0 {
		t.Errorf("JobLog = %v after %v; want ErrNoLog and nothing asked", err, *seen)
	}
}

func TestJobLogReadsASlowLongLogToItsEnd(t *testing.T) {
	t.Parallel()

	// Arrange
	// Fifty megabytes, sent a megabyte at a time: the whole takes longer than
	// the client's timeout, though no part of it is ever long in coming.
	const megabytes = 50

	chunk := strings.Repeat("filler line\n", (1<<20)/len("filler line\n"))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/plain")
		flusher, _ := writer.(http.Flusher)

		for range megabytes {
			_, _ = writer.Write([]byte(chunk))

			flusher.Flush()
			time.Sleep(30 * time.Millisecond)
		}

		_, _ = writer.Write([]byte("the real end\n"))
	}))
	t.Cleanup(server.Close)

	client := forge.New(httpx.Client(time.Second).Do, server.URL, secret)

	// Act
	log, err := client.JobLog(t.Context(), gitlabRepo(), forge.Check{ID: "501", LogAvailable: true})

	// Assert
	if err != nil || !strings.HasSuffix(log.Text, "\nthe real end") {
		t.Errorf("JobLog ends %q, %v; want the log's last line", log.Text[max(0, len(log.Text)-40):], err)
	}
}

// storageBehind is a client whose forge answers a job's log with a redirect
// to location, and whose transport hands every other request — the one to the
// storage — to storage, recording each address asked.
func storageBehind(location string, storage forge.Doer) (forge.Client, *heard) {
	asked := &heard{}

	transport := func(request *http.Request) (*http.Response, error) {
		asked.note(request)

		if request.URL.Host != "api.example.com" {
			return storage(request)
		}

		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": {location}},
			Body:       http.NoBody,
		}, nil
	}

	return forge.New(transport, "https://api.example.com", secret), asked
}

// storedLog is a storage answering with body.
func storedLog(body io.Reader) forge.Doer {
	return func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(body)}, nil
	}
}

func TestJobLogRefusesARedirectToNoAddressAtAll(t *testing.T) {
	t.Parallel()

	// Arrange
	// Go's own client never hands such an answer on, but the transport is a
	// seam, and one that did must not have the address guessed at.
	client, asked := storageBehind("https://storage.example.com/%zz", storedLog(strings.NewReader("log")))

	// Act
	_, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if !errors.Is(err, forge.ErrInsecureLog) || len(asked.paths) != 1 {
		t.Errorf("JobLog = %v after asking %v; want ErrInsecureLog and the storage never asked", err, asked.paths)
	}
}

func TestJobLogKeepsTheStoragesAddressOutOfItsUnreachableError(t *testing.T) {
	t.Parallel()

	// Arrange
	// The storage's address is signed, so it is as much a credential as the
	// token is.
	unreachable := func(*http.Request) (*http.Response, error) { return nil, errBrokeOff }
	client, _ := storageBehind("https://storage.example.com/log?signature=signed-for-this-job", unreachable)

	// Act
	_, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if !errors.Is(err, forge.ErrUnreachable) || !errors.Is(err, errBrokeOff) {
		t.Errorf("JobLog = %v, want ErrUnreachable for why the storage was not reached", err)
	}

	if err != nil && (strings.Contains(err.Error(), "storage.example.com") || strings.Contains(err.Error(), "signed")) {
		t.Errorf("JobLog = %q, want the storage's signed address left out", err)
	}
}

func TestJobLogSaysALogThatBreaksOffWasNotRead(t *testing.T) {
	t.Parallel()

	// Arrange
	// The log's start arrives, then the connection drops: what came is no
	// end of the log, so none of it is shown as one.
	breaking := io.MultiReader(strings.NewReader("--- FAIL: TestRetry\n"), brokenBody{})
	client, _ := storageBehind("https://storage.example.com/log", storedLog(breaking))

	// Act
	log, err := client.JobLog(t.Context(), githubRepo(), failedRun())

	// Assert
	if !errors.Is(err, errBrokeOff) || !strings.Contains(err.Error(), "reading the log") || log.Text != "" {
		t.Errorf("JobLog = %+v, %v; want no log and why it broke off", log, err)
	}
}
