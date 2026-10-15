// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package doctor_test

import (
	"encoding/json"
	"path/filepath"
	"slices"
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
