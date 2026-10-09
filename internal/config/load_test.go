// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"testing/iotest"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestTheWorkingDirectoryFileLayersOverTheHomeFile(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	homeDir := t.TempDir()

	wantPath := write(t, workDir, `{"jira": {"project": "WORK"}}`)
	write(t, homeDir, completeConfig)

	// Act
	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if cfg.Path != wantPath {
		t.Errorf("saves to %s, want the working directory's %s", cfg.Path, wantPath)
	}

	if cfg.Jira.Project != "WORK" || cfg.Jira.Token != "jira-token-1234" {
		t.Errorf("jira = %q, %q; want the working directory's project over the home file's token",
			cfg.Jira.Project, cfg.Jira.Token.Reveal())
	}

	if cfg.Messaging.ClientID != "1234.5678" {
		t.Errorf("messaging.client_id = %q, want the home file's", cfg.Messaging.ClientID)
	}
}

func TestLoadFallsBackToHome(t *testing.T) {
	t.Parallel()

	// Arrange
	workDir := t.TempDir()
	homeDir := t.TempDir()
	wantPath := write(t, homeDir, completeConfig)

	// Act
	cfg, err := config.Load(workDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want %s", cfg.Path, wantPath)
	}

	if cfg.Messaging.Channel != devChannel {
		t.Errorf("messaging.channel = %q, want #dev", cfg.Messaging.Channel)
	}
}

func TestLoadFindsTheRepositoryConfigFromASubdirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	// A repository whose config sits at its root, with work happening in a
	// subdirectory below it. The home directory holds a different file.
	repoRoot := t.TempDir()

	err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repo marker: %v", err)
	}

	wantPath := write(t, repoRoot, `{"jira": {"base_url": "https://repo.example.com"}}`)

	subDir := filepath.Join(repoRoot, "internal", "deep")

	err = os.MkdirAll(subDir, 0o700)
	if err != nil {
		t.Fatalf("making the subdirectory: %v", err)
	}

	homeDir := t.TempDir()
	write(t, homeDir, completeConfig)

	// Act
	cfg, err := config.Load(subDir, homeDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Assert
	// The repository's own file is found, not skipped for the one at home.
	if cfg.Path != wantPath {
		t.Errorf("loaded %s, want the repository's own %s", cfg.Path, wantPath)
	}

	if cfg.Jira.BaseURL != "https://repo.example.com" {
		t.Errorf("jira.base_url = %q, want the repository config's value", cfg.Jira.BaseURL)
	}
}

func TestLoadReportsNotFound(t *testing.T) {
	t.Parallel()

	// Act
	_, err := config.Load(t.TempDir(), t.TempDir())

	// Assert
	if !errors.Is(err, config.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestLoadRejectsMalformedAndUnknownKeys(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"malformed JSON": `{"jira": `,
		// A misspelled key that loaded silently would look exactly like a
		// credential the user never set.
		"unknown key": `{"jiraa": {"token": "x"}}`,
		"unknown nested key": `{"jira": {"base_url": "https://jira.example.com",` +
			` "tokenn": "x"}}`,
		"a request timeout that is not a duration": `{"timing": {"request_timeout": "fast"}}`,
		"a CI interval that is not positive":       `{"timing": {"ci_interval": "0s"}}`,
	}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			workDir := t.TempDir()
			write(t, workDir, contents)

			// Act
			_, err := config.Load(workDir, t.TempDir())

			// Assert
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestLocateIgnoresAnEmptyDirectory(t *testing.T) {
	t.Parallel()

	// A caller with no working directory, or a machine with no home directory,
	// hands Locate an empty string rather than a path. It must skip that entry,
	// not join it into a relative .workflow.json looked up wherever the process
	// happens to run.
	homeWithFile := t.TempDir()
	homeFile := write(t, homeWithFile, completeConfig)

	cases := map[string]struct {
		workDir, homeDir string
		want             config.Files
		wantErr          error
	}{
		"no working directory": {workDir: "", homeDir: homeWithFile, want: config.Files{Home: homeFile}},
		"no home directory":    {workDir: t.TempDir(), homeDir: "", wantErr: config.ErrNotFound},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got, err := config.Locate(tt.workDir, tt.homeDir)

			// Assert
			if got != tt.want || !errors.Is(err, tt.wantErr) {
				t.Errorf("Locate(%q, %q) = %+v, %v, want %+v, %v", tt.workDir, tt.homeDir, got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestALoadReadsTheConfigurationWithItsRevision(t *testing.T) {
	t.Parallel()

	// Arrange
	path := write(t, t.TempDir(), readContents)

	// Act
	cfg, revision, err := config.LoadLayersAt(config.Files{Home: path})
	// Assert
	if err != nil {
		t.Fatalf("LoadLayersAt: %v", err)
	}

	if cfg.Jira.BaseURL != "https://read.example.com" || cfg.Path != path {
		t.Errorf("LoadLayersAt = Jira at %q from %q, want the file's configuration from %q", cfg.Jira.BaseURL, cfg.Path, path)
	}

	if want := revisionOf(t, path); revision != want || !revision.Exists() {
		t.Errorf("LoadLayersAt revision = %v, want the file's, %v", revision, want)
	}
}

func TestALoadOfNoFileIsTheDefaultsAtNoFile(t *testing.T) {
	t.Parallel()

	cases := map[string]func(dir string) string{
		"a missing file": func(dir string) string { return filepath.Join(dir, config.FileName) },
		"an empty path":  func(string) string { return "" },
	}

	for name, pathIn := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			files := config.Files{Home: pathIn(t.TempDir())}
			want := config.Default()
			want.Path, want.Files = files.Home, files

			// Act
			cfg, revision, err := config.LoadLayersAt(files)

			// Assert
			if err != nil || revision != (config.Revision{}) || !reflect.DeepEqual(cfg, want) {
				t.Errorf("LoadLayersAt = %+v at %v, %v; want the defaults at the no-file revision", cfg, revision, err)
			}
		})
	}
}

func TestALoadRefusesAFileThatIsNotValid(t *testing.T) {
	t.Parallel()

	// Arrange
	path := write(t, t.TempDir(), `{"jira": `)

	// Act
	_, _, err := config.LoadLayersAt(config.Files{Home: path})

	// Assert
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("LoadLayersAt = %v, want ErrInvalid", err)
	}
}

func TestALoadReportsAFileItCannotRead(t *testing.T) {
	t.Parallel()

	// Act
	_, _, err := config.LoadLayersAt(config.Files{Home: t.TempDir()})

	// Assert
	if err == nil || errors.Is(err, os.ErrNotExist) || errors.Is(err, config.ErrInvalid) {
		t.Errorf("LoadLayersAt a directory = %v, want the read's own failure", err)
	}
}

func TestALoadReportsAFileItCannotOpen(t *testing.T) {
	t.Parallel()

	// Arrange
	// A regular file where the home directory should be: no file can be
	// opened inside a file, whoever asks, and that is no missing file.
	notADirectory := write(t, t.TempDir(), "{}")
	path := filepath.Join(notADirectory, config.FileName)

	// Act
	_, _, err := config.LoadLayersAt(config.Files{Home: path})

	// Assert
	if !errors.Is(err, syscall.ENOTDIR) || !strings.Contains(err.Error(), path) {
		t.Errorf("LoadLayersAt = %v, want the open's own failure, naming %s", err, path)
	}
}

func TestRepoRootFindsTheEnclosingRepository(t *testing.T) {
	t.Parallel()

	// Arrange
	// A subdirectory below a repository whose root holds .git.
	repoRoot := t.TempDir()

	err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o700)
	if err != nil {
		t.Fatalf("making the repo marker: %v", err)
	}

	subDir := filepath.Join(repoRoot, "internal", "deep")

	err = os.MkdirAll(subDir, 0o700)
	if err != nil {
		t.Fatalf("making the subdirectory: %v", err)
	}

	// Act
	got := config.RepoRoot(subDir)

	// Assert
	// A config written here is found from any subdirectory below it.
	if got != repoRoot {
		t.Errorf("RepoRoot(%q) = %q, want the repository root %q", subDir, got, repoRoot)
	}
}

func TestParseAcceptsAValidConfig(t *testing.T) {
	t.Parallel()

	// Act
	cfg, err := config.Parse(strings.NewReader(`{"jira": {"base_url": "https://jira.example.com"}}`))
	// Assert
	if err != nil {
		t.Fatalf("Parse returned %v, want nil", err)
	}

	if cfg.Jira.BaseURL != "https://jira.example.com" {
		t.Errorf("jira.base_url = %q, want the parsed value", cfg.Jira.BaseURL)
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"an unknown key":        `{"jiraa": {"token": "x"}}`,
		"a bad request timeout": `{"timing": {"request_timeout": "soon"}}`,
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := config.Parse(strings.NewReader(body))

			// Assert
			if !errors.Is(err, config.ErrInvalid) {
				t.Errorf("error = %v, want ErrInvalid", err)
			}
		})
	}
}

// errReadFailed is the failure of a reader that cannot be read.
var errReadFailed = errors.New("the read failed")

func TestParseReportsAReaderThatFails(t *testing.T) {
	t.Parallel()

	// Act
	_, err := config.Parse(iotest.ErrReader(errReadFailed))

	// Assert
	if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, errReadFailed) {
		t.Errorf("Parse = %v, want ErrInvalid wrapping the read's own failure", err)
	}
}

func TestRepoRootOutsideARepositoryIsTheDirectoryItself(t *testing.T) {
	t.Parallel()

	// Arrange
	dir := t.TempDir()

	// Act
	got := config.RepoRoot(dir)

	// Assert
	if got != dir {
		t.Errorf("RepoRoot(%q) = %q, want the directory unchanged", dir, got)
	}
}

func TestParseRefusesAnAddressOrAForgeItCannotUse(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want error
	}{
		"a base URL with a login": {
			body: `{"jira": {"base_url": "https://u:p@jira.example.com"}}`, want: config.ErrCredentialInBaseURL,
		},
		"a base URL that is no address": {
			body: `{"jira": {"base_url": "jira.example.com"}}`, want: config.ErrInvalidBaseURL,
		},
		"a forge kind naming no forge": {body: `{"forge": {"kind": "gitlub"}}`, want: config.ErrUnknownForgeKind},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			_, err := config.Parse(strings.NewReader(tt.body))

			// Assert
			if !errors.Is(err, config.ErrInvalid) || !errors.Is(err, tt.want) {
				t.Errorf("Parse = %v, want ErrInvalid wrapping %v", err, tt.want)
			}
		})
	}
}

func TestParseTakesEitherForgeByName(t *testing.T) {
	t.Parallel()

	// Read as the forge reads it: without case or surrounding space.
	for _, kind := range []string{"", "GitHub", " gitlab "} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			// Act
			cfg, err := config.Parse(strings.NewReader(`{"forge": {"kind": "` + kind + `", "host": "git.example.com"}}`))

			// Assert
			if err != nil || cfg.Forge.Kind != kind {
				t.Errorf("Parse = forge.kind %q, %v; want %q read", cfg.Forge.Kind, err, kind)
			}
		})
	}
}
