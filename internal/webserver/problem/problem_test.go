// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package problem_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/webserver/problem"
)

// errorsPage is where every problem's type URI points, one anchor per code.
const errorsPage = "https://jacob-delgado.github.io/workflow/docs/errors/"

// problemJSON is the media type every problem is written in.
const problemJSON = "application/problem+json"

// errUnparsable is a parser's own failure, in the words a caller must never
// see.
var errUnparsable = errors.New(`strconv.ParseInt: parsing "soon": invalid syntax`)

func TestNewFixesEachCodesStatusTitleAndType(t *testing.T) {
	t.Parallel()

	cases := []struct {
		code     api.ProblemCode
		status   int
		title    string
		fragment string
	}{
		{api.ProblemCodeBadRequest, http.StatusBadRequest, "Bad request", "bad-request"},
		{api.ProblemCodeUnauthorized, http.StatusUnauthorized, "Unauthorized", "unauthorized"},
		{api.ProblemCodeNotFound, http.StatusNotFound, "Not found", "not-found"},
		{api.ProblemCodeMethodNotAllowed, http.StatusMethodNotAllowed, "Method not allowed", "method-not-allowed"},
		{api.ProblemCodeConflict, http.StatusConflict, "Conflict", "conflict"},
		{api.ProblemCodeUnprocessable, http.StatusUnprocessableEntity, "Unprocessable content", "unprocessable"},
		{api.ProblemCodeNotSetUp, http.StatusUnprocessableEntity, "Not set up", "not-set-up"},
		{api.ProblemCodeTooLong, http.StatusUnprocessableEntity, "Too long", "too-long"},
		{
			api.ProblemCodePreconditionRequired, http.StatusPreconditionRequired, "Precondition required",
			"precondition-required",
		},
		{api.ProblemCodeUnreachable, http.StatusBadGateway, "Upstream unreachable", "unreachable"},
		{api.ProblemCodeRateLimited, http.StatusServiceUnavailable, "Rate limited", "rate-limited"},
		{api.ProblemCodeFetchFailed, http.StatusBadGateway, "Fetch failed", "fetch-failed"},
		{api.ProblemCodeCheckFailed, http.StatusUnprocessableEntity, "Check failed", "check-failed"},
		{api.ProblemCodeCiFailed, http.StatusConflict, "CI failed", "ci-failed"},
		{api.ProblemCodeInternal, http.StatusInternalServerError, "Internal error", "internal"},
	}

	for _, test := range cases {
		t.Run(string(test.code), func(t *testing.T) {
			t.Parallel()

			// Act
			prob := problem.New(test.code, "what happened")

			// Assert
			want := api.Problem{
				Type: errorsPage + "#" + test.fragment, Title: test.title, Status: test.status,
				Detail: "what happened", Code: test.code,
			}
			if !reflect.DeepEqual(prob, want) {
				t.Errorf("New(%q) = %+v, want %+v", test.code, prob, want)
			}
		})
	}
}

func TestAnswerIsTheProblemAtItsStatusWithItsWait(t *testing.T) {
	t.Parallel()

	// Arrange
	prob := problem.New(api.ProblemCodeRateLimited, "the forge asked to wait")
	prob.RetryAfter = new(30)

	// Act
	answer := problem.Answer[api.GetAnnouncementdefaultApplicationProblemPlusJSONResponse](prob)

	// Assert
	if answer.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", answer.StatusCode, http.StatusServiceUnavailable)
	}

	if wait := answer.Headers.RetryAfter; wait == nil || *wait != 30 {
		t.Errorf("Retry-After = %v, want 30", wait)
	}

	if !reflect.DeepEqual(answer.Body, prob) {
		t.Errorf("body = %+v, want %+v", answer.Body, prob)
	}
}

func TestAnswerWithoutAWaitHasNoRetryAfter(t *testing.T) {
	t.Parallel()

	// Arrange
	prob := problem.New(api.ProblemCodeConflict, "the branch already exists")

	// Act
	answer := problem.Answer[api.AnnouncedefaultApplicationProblemPlusJSONResponse](prob)

	// Assert
	if answer.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", answer.StatusCode, http.StatusConflict)
	}

	if wait := answer.Headers.RetryAfter; wait != nil {
		t.Errorf("Retry-After = %d, want none without a wait", *wait)
	}
}

func TestWriteSendsTheCodesProblemAsProblemJSON(t *testing.T) {
	t.Parallel()

	// Arrange
	recorder := httptest.NewRecorder()

	// Act
	problem.Write(recorder, api.ProblemCodeNotFound, "no such endpoint")

	// Assert
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	if kind := recorder.Header().Get("Content-Type"); kind != problemJSON {
		t.Errorf("Content-Type = %q, want %s", kind, problemJSON)
	}

	want := problem.New(api.ProblemCodeNotFound, "no such endpoint")
	if sent := decode(t, recorder); !reflect.DeepEqual(sent, want) {
		t.Errorf("body = %+v, want %+v", sent, want)
	}
}

func TestWriteRequestErrorKeepsTheParsersWordsOffTheWire(t *testing.T) {
	t.Parallel()

	// Arrange
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/issues?limit=soon", http.NoBody)

	// Act
	problem.WriteRequestError(recorder, request, errUnparsable)

	// Assert
	if strings.Contains(recorder.Body.String(), "strconv") {
		t.Errorf("body = %s, want none of the parser's words", recorder.Body.String())
	}

	want := problem.New(api.ProblemCodeBadRequest, "the request could not be understood")
	if sent := decode(t, recorder); !reflect.DeepEqual(sent, want) {
		t.Errorf("body = %+v, want %+v", sent, want)
	}
}

// decode reads the problem a recorder was sent.
func decode(t *testing.T, recorder *httptest.ResponseRecorder) api.Problem {
	t.Helper()

	var prob api.Problem

	err := json.NewDecoder(recorder.Body).Decode(&prob)
	if err != nil {
		t.Fatalf("decode %q: %v", recorder.Body.String(), err)
	}

	return prob
}
