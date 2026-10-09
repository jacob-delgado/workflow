// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package setup_test

import (
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/jira"
	"github.com/jacob-delgado/workflow/internal/setup"
)

// jiraAddress is the Jira address these tests answer with.
const jiraAddress = "https://jira.example.com"

// typedToken is the Jira token these tests type.
const typedToken = "typed-jira-token"

// jiraAnswering is a Jira that answers who the token is with status and body,
// recording the Authorization header it was sent.
func jiraAnswering(status int, body string, sent *string) jira.Doer {
	return func(request *http.Request) (*http.Response, error) {
		if sent != nil {
			*sent = request.Header.Get("Authorization")
		}

		return &http.Response{
			StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}},
			Body: io.NopCloser(strings.NewReader(body)), Request: request,
		}, nil
	}
}

// acceptingJira is a Jira that knows the token as Fred.
func acceptingJira() jira.Doer {
	return jiraAnswering(http.StatusOK, `{"displayName":"Fred F. User","name":"fred"}`, nil)
}

// guideIn is a guide for a fresh working directory and home, with no keychain.
func guideIn(t *testing.T, doer jira.Doer) setup.Guide {
	t.Helper()

	return setup.Guide{Where: setup.Where{WorkDir: t.TempDir(), HomeDir: t.TempDir()}, Doer: doer}
}

// answered is a request for place with Jira's address and the typed token.
func answered(place setup.Place) setup.Request {
	return setup.Request{
		Place: place,
		Answers: setup.Answers{
			Jira: config.Jira{BaseURL: jiraAddress, Token: typedToken},
		},
	}
}

func TestWriteRefusesAFileAlreadyThere(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	path := guide.Where.Path(setup.Home)

	err := os.WriteFile(path, []byte(`{"jira":{"base_url":"https://kept.example.com"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the file already there: %v", err)
	}

	// Act
	_, err = guide.Write(t.Context(), answered(setup.Home))

	// Assert
	kept, readErr := os.ReadFile(path)
	if !errors.Is(err, setup.ErrExists) || readErr != nil || !strings.Contains(string(kept), "kept.example.com") {
		t.Errorf("Write over a file = %v, left %q (%v); want it refused and the file kept", err, kept, readErr)
	}
}

func TestWriteSavesTheWebhookAsSlacks(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	request := setup.Request{Place: setup.Home, Answers: setup.Answers{Webhook: "https://hooks.slack.com/x"}}

	// Act
	written, err := guide.Write(t.Context(), request)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert
	cfg, _, err := config.LoadLayersAt(config.Files{Home: written.Path})
	if err != nil || cfg.Messaging.Kind != config.KindSlack || cfg.Messaging.WebhookURL != request.Answers.Webhook {
		t.Errorf("wrote messaging %+v (%v), want the webhook as Slack's", cfg.Messaging, err)
	}
}

func TestWhereNamesTheRepositoryRootAndTheHomeDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	root := t.TempDir()

	err := os.Mkdir(filepath.Join(root, ".git"), 0o750)
	if err != nil {
		t.Fatalf("making the repository: %v", err)
	}

	inside := filepath.Join(root, "cmd")

	err = os.Mkdir(inside, 0o750)
	if err != nil {
		t.Fatalf("making a subdirectory: %v", err)
	}

	where := setup.Where{WorkDir: inside, HomeDir: t.TempDir()}

	// Act
	repository, home := where.Path(setup.Repository), where.Path(setup.Home)

	// Assert
	if repository != filepath.Join(root, config.FileName) || home != filepath.Join(where.HomeDir, config.FileName) {
		t.Errorf("paths = %q and %q, want the repository root's and the home directory's", repository, home)
	}
}

func TestCheckNamesWhoTheTokenAuthenticatesAs(t *testing.T) {
	t.Parallel()

	// Arrange
	var sent string

	guide := guideIn(t, jiraAnswering(http.StatusOK, `{"displayName":"Fred F. User","name":"fred"}`, &sent))

	// Act
	who, err := guide.Check(t.Context(), answered(setup.Home).Answers.Jira)

	// Assert
	if err != nil || who != "Fred F. User (fred)" || sent != "Bearer "+typedToken {
		t.Errorf("Check = %q, %v (sent %q), want Fred named over the typed token", who, err, sent)
	}
}

func TestCheckReportsATokenJiraRefuses(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, jiraAnswering(http.StatusUnauthorized, `{}`, nil))

	// Act
	_, err := guide.Check(t.Context(), answered(setup.Home).Answers.Jira)

	// Assert
	if !errors.Is(err, jira.ErrUnauthorized) {
		t.Errorf("Check = %v, want %v", err, jira.ErrUnauthorized)
	}
}

func TestOfferNamesBothPlacesAndTheKeychainForEach(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	guide.StoreSecret = (&keychain{}).store

	// Act
	offer := guide.Offer()

	// Assert
	want := []setup.Destination{
		{Place: setup.Repository, Path: guide.Where.Path(setup.Repository), Keychain: true},
		{Place: setup.Home, Path: guide.Where.Path(setup.Home), Keychain: true},
	}
	if !slices.Equal(offer.Places, want) {
		t.Errorf("Offer = %+v, want the repository then home, the keychain for each", offer.Places)
	}
}

func TestOfferNamesTheKeychainForARepositoryRootedAtHome(t *testing.T) {
	t.Parallel()

	// Arrange
	home := t.TempDir()
	guide := setup.Guide{Where: setup.Where{WorkDir: home, HomeDir: home}, StoreSecret: (&keychain{}).store}

	// Act
	offer := guide.Offer()

	// Assert
	if !offer.Places[0].Keychain {
		t.Errorf("Offer = %+v, want the keychain for the repository's file, which is the home file", offer.Places)
	}
}

func TestWriteInARepositoryLiesOverTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	home := guide.Where.Path(setup.Home)

	err := os.WriteFile(home, []byte(`{"jira":{"base_url":"https://home.example.com","project":"HOME"}}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}

	// Act
	written, err := guide.Write(t.Context(), answered(setup.Repository))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Assert
	cfg, _, err := config.LoadLayersAt(guide.Where.Layers(setup.Repository))
	if err != nil || cfg.Jira.Project != "HOME" || cfg.Jira.BaseURL != jiraAddress {
		t.Errorf("read %+v (%v) over %s, want the repository's answers over the home file's project",
			cfg.Jira, err, written.Path)
	}
}

func TestWriteNamesAFileGitWouldCommit(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())

	err := exec.CommandContext(t.Context(), "git", "init", "-q", guide.Where.WorkDir).Run()
	if err != nil {
		t.Fatalf("making the repository: %v", err)
	}

	// Act
	written, err := guide.Write(t.Context(), answered(setup.Repository))

	// Assert
	if err != nil || !written.NotIgnored {
		t.Errorf("Write = %+v, %v; want the file named as one git would commit", written, err)
	}
}

func TestCheckNamesALoginWhereJiraWithholdsTheName(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, jiraAnswering(http.StatusOK, `{"name":"fred"}`, nil))

	// Act
	who, err := guide.Check(t.Context(), answered(setup.Home).Answers.Jira)

	// Assert
	if err != nil || who != "fred" {
		t.Errorf("Check = %q, %v; want the login", who, err)
	}
}

func TestOfferNamesNoKeychainWhereThereIsNone(t *testing.T) {
	t.Parallel()

	// Act
	offer := guideIn(t, acceptingJira()).Offer()

	// Assert
	if slices.ContainsFunc(offer.Places, func(place setup.Destination) bool { return place.Keychain }) {
		t.Errorf("Offer = %+v, want no keychain for any place", offer.Places)
	}
}

func TestLayersStandAloneWhereTheRepositoryIsHome(t *testing.T) {
	t.Parallel()

	// Arrange
	home := t.TempDir()

	err := os.WriteFile(filepath.Join(home, config.FileName), []byte(`{}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing the home file: %v", err)
	}

	// Act
	layers := setup.Where{WorkDir: home, HomeDir: home}.Layers(setup.Repository)

	// Assert
	if layers != (config.Files{}) {
		t.Errorf("Layers = %+v, want the file to stand alone", layers)
	}
}

func TestWriteRefusesALinkWhereTheFileWouldGo(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := guideIn(t, acceptingJira())
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.json")

	err := os.Symlink(elsewhere, guide.Where.Path(setup.Home))
	if err != nil {
		t.Fatalf("linking the file's path elsewhere: %v", err)
	}

	// Act
	_, err = guide.Write(t.Context(), answered(setup.Home))

	// Assert
	_, statErr := os.Lstat(elsewhere)
	if !errors.Is(err, setup.ErrExists) || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Write over a dangling link = %v, and the link's target %v; want it refused and nothing written there",
			err, statErr)
	}
}

func TestCreateWritesNothingThroughALink(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)
	elsewhere := filepath.Join(t.TempDir(), "elsewhere.json")

	err := os.Symlink(elsewhere, path)
	if err != nil {
		t.Fatalf("linking the file's path elsewhere: %v", err)
	}

	// Act
	err = setup.Create(path, config.Files{}, config.Default(), config.Revision{})

	// Assert
	_, statErr := os.Lstat(elsewhere)
	if err == nil || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Create through a dangling link = %v, and the link's target %v; want it refused and nothing written there",
			err, statErr)
	}
}

func TestCreateWritesAPrivateFile(t *testing.T) {
	t.Parallel()

	// Arrange
	path := filepath.Join(t.TempDir(), config.FileName)

	// Act
	err := setup.Create(path, config.Files{}, config.Default(), config.Revision{})

	// Assert
	info, statErr := os.Lstat(path)
	if err != nil || statErr != nil || info.Mode() != config.FileMode {
		t.Errorf("Create = %v, left %v (%v); want a regular file at mode %#o", err, info, statErr, config.FileMode)
	}
}

func TestOfferLeavesHomeOutWithoutAHomeDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	guide := setup.Guide{Where: setup.Where{WorkDir: t.TempDir()}, Doer: acceptingJira()}

	// Act
	offer := guide.Offer()

	// Assert
	if len(offer.Places) != 1 || offer.Places[0].Place != setup.Repository {
		t.Errorf("Offer without a home directory = %+v, want the repository alone", offer.Places)
	}
}

//nolint:paralleltest // t.Chdir moves the whole process, so this runs serially.
func TestWriteRefusesHomeWithoutAHomeDirectory(t *testing.T) {
	// Arrange
	workDir := t.TempDir()
	t.Chdir(workDir)

	guide := setup.Guide{Where: setup.Where{WorkDir: t.TempDir()}, Doer: acceptingJira()}

	// Act
	_, err := guide.Write(t.Context(), answered(setup.Home))

	// Assert
	_, statErr := os.Lstat(filepath.Join(workDir, config.FileName))
	if !errors.Is(err, setup.ErrNoHome) || !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("Write home without a home directory = %v, and the working directory's file %v; "+
			"want ErrNoHome, nothing written", err, statErr)
	}
}

//nolint:paralleltest // t.Chdir moves the whole process, so this runs serially.
func TestLayersStandAloneWithoutAHomeDirectory(t *testing.T) {
	// Arrange
	workDir := t.TempDir()
	t.Chdir(workDir)

	err := os.WriteFile(filepath.Join(workDir, config.FileName), []byte(`{}`), config.FileMode)
	if err != nil {
		t.Fatalf("writing a file in the working directory: %v", err)
	}

	// Act
	layers := setup.Where{WorkDir: t.TempDir()}.Layers(setup.Repository)

	// Assert
	if layers != (config.Files{}) {
		t.Errorf("Layers without a home directory = %+v, want the file to stand alone", layers)
	}
}

func TestKeepableKeepsOnlyAFailureJiraMayAnswerLater(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		baseURL string
		status  int
		want    bool
	}{
		"an address that is not one":      {baseURL: "ftp://jira.example.com", status: http.StatusOK, want: false},
		"an address carrying a password":  {baseURL: "https://fred:pw@jira.example.com", status: http.StatusOK, want: false},
		"a token Jira refuses for now":    {baseURL: jiraAddress, status: http.StatusUnauthorized, want: true},
		"a Jira that fails as it answers": {baseURL: jiraAddress, status: http.StatusBadGateway, want: true},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			guide := guideIn(t, jiraAnswering(tt.status, `{}`, nil))

			_, err := guide.Check(t.Context(), config.Jira{BaseURL: tt.baseURL, Token: typedToken})
			if err == nil {
				t.Fatal("Check passed, want it to fail")
			}

			// Act
			keepable := setup.Keepable(err)

			// Assert
			if keepable != tt.want {
				t.Errorf("Keepable(%v) = %t, want %t", err, keepable, tt.want)
			}
		})
	}
}

func TestWriteOverAHomeFileItCannotReadWritesNothing(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		contents string
		mode     os.FileMode
		want     error
	}{
		"a home file others may write": {
			contents: `{"jira":{"project":"HOME"}}`, mode: 0o666, want: config.ErrUntrustedFile,
		},
		"a home file that is no configuration": {
			contents: `{"jira":`, mode: config.FileMode, want: config.ErrInvalid,
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			guide := guideIn(t, acceptingJira())
			home := guide.Where.Path(setup.Home)
			writeHomeFile(t, guide, tt.contents)

			// os.WriteFile leaves the mode to the umask; the case sets its own.
			err := os.Chmod(home, tt.mode)
			if err != nil {
				t.Fatalf("setting the home file's mode: %v", err)
			}

			// Act
			_, err = guide.Write(t.Context(), answered(setup.Repository))

			// Assert
			if !errors.Is(err, tt.want) {
				t.Errorf("Write = %v, want %v", err, tt.want)
			}

			_, statErr := os.Lstat(guide.Where.Path(setup.Repository))
			if !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("the repository's file is there (%v), want nothing written", statErr)
			}
		})
	}
}
