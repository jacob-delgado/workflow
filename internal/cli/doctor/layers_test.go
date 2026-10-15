// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
	"github.com/jacob-delgado/workflow/internal/config"
)

func TestDoctorNamesEveryFileInEffect(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.HomeAndRepository(t)
	clitest.WriteFile(t, where.Dir, `{"jira": {"project": "OSS"}}`)

	// Act
	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor", "--json")

	// Assert
	var report struct {
		Configuration struct {
			Path  string   `json:"path"`
			Files []string `json:"files"`
		} `json:"configuration"`
	}

	err := json.Unmarshal([]byte(printed.Stdout), &report)

	home, repo := filepath.Join(where.Home, config.FileName), filepath.Join(where.Dir, config.FileName)
	got := report.Configuration

	if err != nil || got.Path != repo || !slices.Equal(got.Files, []string{home, repo}) {
		t.Errorf("configuration = %+v, %v; want saving to %s, read from %s then %s", got, err, repo, home, repo)
	}
}

func TestDoctorNamesTheFirstFileOthersCanReach(t *testing.T) {
	t.Parallel()

	// Arrange
	// Both files are open to other users; the home file is read first, so it
	// is the one doctor names.
	where := clitest.HomeAndRepository(t)
	home := filepath.Join(where.Home, config.FileName)
	repo := clitest.WriteFile(t, where.Dir, `{"jira": {"project": "OSS"}}`)

	for _, path := range []string{home, repo} {
		err := os.Chmod(path, 0o644)
		if err != nil {
			t.Fatalf("chmod %s: %v", path, err)
		}
	}

	// Act
	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor")

	// Assert
	if !strings.Contains(printed.Stdout, "chmod 600 "+home+"`") ||
		strings.Contains(printed.Stdout, "chmod 600 "+repo) {
		t.Errorf("doctor does not name the home file, the first others can reach:\n%s", printed.Stdout)
	}
}
