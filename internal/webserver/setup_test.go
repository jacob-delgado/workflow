// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/seams"
	"github.com/jacob-delgado/workflow/internal/setup"
	"github.com/jacob-delgado/workflow/internal/webserver"
)

// Answers the page's first run sends.
const (
	setupPath      = "/api/config/setup"
	setupJira      = "https://jira.internal.example"
	setupToken     = "typed-in-the-page-5150"
	setupKeychain  = "security find-generic-password -s workflow-jira -w"
	setupAnswering = `{"displayName":"Fred F. User","name":"fred"}`
)

// firstRun is a server with no configuration file, over a setup that writes
// into directories of the test's own and checks with a Jira answering with
// status; reached records each directory the server took up again.
type firstRun struct {
	where   setup.Where
	status  int
	stored  string
	reached []string
	noted   []string
}

// newFirstRun is a first run over a Jira that answers with status.
func newFirstRun(t *testing.T, status int) *firstRun {
	t.Helper()

	return &firstRun{where: setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}, status: status}
}

// jira answers who the token is with the run's status.
func (r *firstRun) jira(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: r.status, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(setupAnswering)), Request: request,
	}, nil
}

// deps wires the run's setup, the keychain, and a way to take the file up.
func (r *firstRun) deps(t *testing.T) webserver.Deps {
	t.Helper()

	guide := setup.Guide{Where: r.where, Doer: jira.Doer(r.jira), StoreSecret: func(secret string) (string, error) {
		r.stored = secret

		return setupKeychain, nil
	}}

	deps := webserver.Deps{
		Repositories: seams.Repositories{Here: seams.Place{Dir: r.where.WorkDir}, Home: r.where.HomeDir},
		Setup: seams.Setup{
			Offer: guide.Offer,
			Check: func(settings config.Jira) (string, error) { return guide.Check(t.Context(), settings) },
			Write: func(request setup.Request) (setup.Written, error) { return guide.Write(t.Context(), request) },
		},
		Unexpected: func(_ config.Config, err error) { r.noted = append(r.noted, err.Error()) },
	}
	deps.Reach = func(dir string) (webserver.World, error) {
		r.reached = append(r.reached, dir)

		cfg, err := config.Load(dir, r.where.HomeDir)
		next := deps
		next.Setup = seams.Setup{}

		return webserver.World{Deps: next, Config: cfg, Info: webserver.Info{Version: testVersion}}, err
	}

	return deps
}

// handler is the run's server.
func (r *firstRun) handler(t *testing.T) http.Handler {
	t.Helper()

	return serve(t, r.deps(t), config.Default())
}

// setupBody is a request to set up in the repository with the typed token,
// kept in the keychain when keychain says, and unchecked when keep says.
func setupBody(t *testing.T, keychain, keep bool) string {
	t.Helper()

	return setupBodyAt(t, api.SetupPlaceNameRepository, keychain, keep)
}

// setupBodyAt is setupBody for the file at place.
func setupBodyAt(t *testing.T, place api.SetupPlaceName, keychain, keep bool) string {
	t.Helper()

	body, err := json.Marshal(api.SetupRequest{
		Place: place, JiraBaseURL: setupJira, JiraToken: setupToken,
		WebhookURL: "", Keychain: keychain, KeepUnchecked: keep,
	})
	if err != nil {
		t.Fatalf("encoding the request: %v", err)
	}

	return string(body)
}

func TestSetupIsOfferedWhereNoFileApplies(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)

	// Act
	offer := decode[api.SetupOffer](t, get(t, run.handler(t), setupPath))

	// Assert
	if !offer.Needed || len(offer.Places) != 2 || offer.Places[0].Place != api.SetupPlaceNameRepository ||
		offer.Places[0].Keychain || offer.Places[1].Shown != "~/"+config.FileName || !offer.Places[1].Keychain {
		t.Errorf("offer = %+v, want setup needed, the repository first and home shown from home, "+
			"the keychain for home alone", offer)
	}
}

func TestTheConfigurationIsNotFoundWhereNoFileApplies(t *testing.T) {
	t.Parallel()

	// Act
	recorder := get(t, newFirstRun(t, http.StatusOK).handler(t), "/api/config")

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusNotFound || failure.Code != api.ProblemCodeNotFound {
		t.Errorf("status %d, problem %+v; want 404 not_found", recorder.Code, failure)
	}
}

func TestSetupWritesTheFileAndServesItWithoutTheToken(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBodyAt(t, api.SetupPlaceNameHome, true, false))

	// Assert
	result := decode[api.SetupResult](t, recorder)
	if recorder.Code != http.StatusOK || result.JiraUser != "Fred F. User (fred)" || !result.Keychain ||
		!result.Reopened || result.Shown == "" {
		t.Fatalf("status %d, result %+v; want the file written, the user named and the server reopened",
			recorder.Code, result)
	}

	contents, err := os.ReadFile(result.Path)
	if err != nil || strings.Contains(string(contents), setupToken) || run.stored != setupToken {
		t.Errorf("wrote %q (%v), keychain %q; want the token in the keychain alone", contents, err, run.stored)
	}
}

func TestAfterASetupTheConfigurationIsServedMasked(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	handler := run.handler(t)
	send(t, handler, http.MethodPost, setupPath, setupBody(t, false, false))

	// Act
	read := get(t, handler, "/api/config")

	// Assert
	body := read.Body.String()
	if read.Code != http.StatusOK || !strings.Contains(body, setupJira) || strings.Contains(body, setupToken) {
		t.Errorf("after the setup GET /api/config = %d %s, want the file served, its token masked", read.Code, body)
	}
}

func TestSetupKeepsTheTokenInTheFileWhenTheKeychainIsDeclined(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	result := decode[api.SetupResult](t, recorder)

	cfg, _, err := config.LoadLayersAt(config.Files{Home: result.Path})
	if err != nil || cfg.Jira.Token.Reveal() != setupToken || run.stored != "" {
		t.Errorf("wrote %+v (%v), keychain %q; want the token in the file alone", cfg.Jira, err, run.stored)
	}
}

func TestSetupNeverAnswersOrNotesTheToken(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusOK, http.StatusUnauthorized, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := newFirstRun(t, status)

			// Act
			recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, false, false))

			// Assert
			noted := strings.Join(run.noted, " ")
			if strings.Contains(recorder.Body.String(), setupToken) || strings.Contains(noted, setupToken) {
				t.Errorf("the answer %s or the notes %q carry the token", recorder.Body.String(), noted)
			}
		})
	}
}

func TestSetupRefusesATokenJiraDoesNotAccept(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusUnauthorized)

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeCheckFailed ||
		strings.Contains(failure.Detail, "jira.internal.example") {
		t.Errorf("status %d, problem %+v; want check_failed naming no host", recorder.Code, failure)
	}

	_, err := os.Stat(run.where.Path(setup.Repository))
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a refused check left a file: %v", err)
	}
}

func TestSetupKeepsATokenUncheckedWhenAsked(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusUnauthorized)

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, false, true))

	// Assert
	result := decode[api.SetupResult](t, recorder)
	if recorder.Code != http.StatusOK || result.JiraUser != "" {
		t.Errorf("status %d, result %+v; want the file written unchecked", recorder.Code, result)
	}
}

func TestSetupIsRefusedWhereAFileApplies(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	cfg := config.Default()
	cfg.Path = run.where.Path(setup.Home)

	// Act
	recorder := send(t, serve(t, run.deps(t), cfg), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusConflict || failure.Code != api.ProblemCodeConflict {
		t.Errorf("status %d, problem %+v; want 409", recorder.Code, failure)
	}
}

func TestSetupTakesTheFileUpWhereTheServerWorks(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)

	// Act
	send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	if len(run.reached) != 1 || run.reached[0] != run.where.WorkDir {
		t.Errorf("reached %q, want the server to take the file up in %s", run.reached, run.where.WorkDir)
	}
}

func TestSetupTakesUpAFileMadeSinceTheServerStarted(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	handler := run.handler(t)

	err := os.WriteFile(run.where.Path(setup.Repository), []byte(`{"jira":{"project":"BYHAND"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the file by hand: %v", err)
	}

	// Act
	recorder := send(t, handler, http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	read := get(t, handler, "/api/config")
	if recorder.Code != http.StatusConflict || read.Code != http.StatusOK ||
		!strings.Contains(read.Body.String(), "BYHAND") {
		t.Errorf("setup = %d, then GET /api/config = %d %s; want 409 and the file made by hand served",
			recorder.Code, read.Code, read.Body.String())
	}
}

func TestTwoSetupsAtOnceWriteOnce(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	deps := run.deps(t)
	check := deps.Setup.Check

	var asked atomic.Int32

	deps.Setup.Check = func(settings config.Jira) (string, error) {
		asked.Add(1)
		time.Sleep(100 * time.Millisecond)

		return check(settings)
	}
	handler := serve(t, deps, config.Default())

	// Act
	codes := make(chan int, 2)

	var both sync.WaitGroup
	for range 2 {
		both.Go(func() { codes <- send(t, handler, http.MethodPost, setupPath, setupBody(t, false, false)).Code })
	}

	both.Wait()
	close(codes)

	// Assert
	answered := slices.Sorted(func(yield func(int) bool) {
		for code := range codes {
			yield(code)
		}
	})
	if !slices.Equal(answered, []int{http.StatusOK, http.StatusConflict}) || asked.Load() != 1 {
		t.Errorf("two setups at once answered %v, asking Jira %d times; want one written and one 409, Jira asked once",
			answered, asked.Load())
	}
}

func TestSetupWithJiraLeftOutAsksNoOne(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusUnauthorized)
	body := `{"place":"home","jira_base_url":"","jira_token":"","webhook_url":"","keychain":false,"keep_unchecked":false}`

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, body)

	// Assert
	result := decode[api.SetupResult](t, recorder)
	if recorder.Code != http.StatusOK || result.JiraUser != "" || result.Path != run.where.Path(setup.Home) {
		t.Errorf("status %d, result %+v; want the home file written with no Jira", recorder.Code, result)
	}
}

func TestSetupNeverWritesOverAFileAtThePathChosen(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	path := run.where.Path(setup.Repository)

	err := os.WriteFile(path, []byte(`{}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the file already there: %v", err)
	}

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	kept, readErr := os.ReadFile(path)
	if recorder.Code != http.StatusConflict || readErr != nil || string(kept) != `{}` {
		t.Errorf("status %d, file %q (%v); want 409 and the file kept", recorder.Code, kept, readErr)
	}
}

func TestSetupRefusesTheKeychainWhereThereIsNone(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	deps := run.deps(t)
	guide := setup.Guide{Where: run.where, Doer: jira.Doer(run.jira)}
	deps.Setup.Write = func(request setup.Request) (setup.Written, error) { return guide.Write(t.Context(), request) }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, setupPath, setupBody(t, true, false))

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable {
		t.Errorf("status %d, problem %+v; want 422 unprocessable", recorder.Code, failure)
	}
}

func TestSetupRefusesTheKeychainForAFileOtherThanTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, setupBody(t, true, false))

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable ||
		!strings.Contains(failure.Detail, "home") || run.stored != "" {
		t.Errorf("status %d, problem %+v, keychain %q; want 422 naming the home file and nothing stored",
			recorder.Code, failure, run.stored)
	}
}

func TestSetupWithNoWayToTakeTheFileUpSaysSo(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	deps := run.deps(t)
	deps.Reach = nil

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	result := decode[api.SetupResult](t, recorder)
	if recorder.Code != http.StatusOK || result.Reopened {
		t.Errorf("status %d, result %+v; want the file written and not taken up", recorder.Code, result)
	}
}

func TestSetupThatCannotBeTakenUpIsNoted(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	deps := run.deps(t)
	deps.Reach = func(string) (webserver.World, error) { return webserver.World{}, errSetupUnreachable }

	// Act
	recorder := send(t, serve(t, deps, config.Default()), http.MethodPost, setupPath, setupBody(t, false, false))

	// Assert
	result := decode[api.SetupResult](t, recorder)
	if result.Reopened || len(run.noted) != 1 {
		t.Errorf("result %+v, noted %q; want it not taken up, and why noted", result, run.noted)
	}
}

// errSetupUnreachable is a directory the server could not wire again.
var errSetupUnreachable = errors.New("the directory could not be wired")

func TestSetupNamesTheCheckThatDidNotPass(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		status  int
		address string
		want    string
	}{
		"no Jira there":   {status: http.StatusNotFound, address: setupJira, want: "no Jira answers"},
		"something else":  {status: http.StatusTeapot, address: setupJira, want: "could not check the token"},
		"Jira is refused": {status: http.StatusForbidden, address: setupJira, want: "Jira did not accept"},
	}

	for name, each := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := newFirstRun(t, each.status)
			body := strings.Replace(setupBody(t, false, false), setupJira, each.address, 1)

			// Act
			recorder := send(t, run.handler(t), http.MethodPost, setupPath, body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			if failure.Code != api.ProblemCodeCheckFailed || !strings.Contains(failure.Detail, each.want) {
				t.Errorf("problem %+v, want check_failed saying %q", failure, each.want)
			}
		})
	}
}

func TestSetupNeverKeepsAnAddressThatIsNotOne(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"not an address":          "jira.example.com",
		"a password inside it":    "https://fred:hunter2@jira.example.com",
		"http to another machine": "http://jira.example.com",
	}

	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			run := newFirstRun(t, http.StatusOK)
			body := strings.Replace(setupBody(t, false, true), setupJira, address, 1)

			// Act
			recorder := send(t, run.handler(t), http.MethodPost, setupPath, body)

			// Assert
			failure := decode[api.Problem](t, recorder)
			_, statErr := os.Stat(run.where.Path(setup.Repository))

			if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable ||
				!strings.Contains(failure.Detail, "not an https address, or http to this machine,") ||
				strings.Contains(failure.Detail, "keep") || strings.Contains(failure.Detail, "hunter2") ||
				!errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("status %d, problem %+v, file %v; want 422 unprocessable naming the address, "+
					"offering no keeping, and nothing written", recorder.Code, failure, statErr)
			}
		})
	}
}

func TestSetupOffersNothingWhereItIsNotWired(t *testing.T) {
	t.Parallel()

	// Act
	offer := decode[api.SetupOffer](t, get(t, serve(t, webserver.Deps{}, config.Default()), setupPath))

	// Assert
	if offer.Needed || len(offer.Places) != 0 {
		t.Errorf("offer = %+v, want nothing offered", offer)
	}
}

func TestSetupRefusesHomeWithoutAHomeDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	run := newFirstRun(t, http.StatusOK)
	run.where.HomeDir = ""

	body, err := json.Marshal(api.SetupRequest{
		Place: api.SetupPlaceNameHome, JiraBaseURL: setupJira, JiraToken: setupToken,
	})
	if err != nil {
		t.Fatalf("encoding the request: %v", err)
	}

	// Act
	recorder := send(t, run.handler(t), http.MethodPost, setupPath, string(body))

	// Assert
	failure := decode[api.Problem](t, recorder)
	if recorder.Code != http.StatusUnprocessableEntity || failure.Code != api.ProblemCodeUnprocessable {
		t.Errorf("status %d, problem %+v; want 422 unprocessable", recorder.Code, failure)
	}
}
