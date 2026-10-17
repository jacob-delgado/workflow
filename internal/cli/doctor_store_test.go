// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/cli/clitest"
)

// storeFacts is the store as doctor --json reports it.
type storeFacts struct {
	Dir      string `json:"dir"`
	Disabled bool   `json:"disabled"`
	Problem  string `json:"problem"`
}

// doctorStore runs doctor --json at where and returns its store field.
func doctorStore(t *testing.T, where clitest.Place) storeFacts {
	t.Helper()

	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor", "--json")

	var report struct {
		Store *storeFacts `json:"store"`
	}

	err := json.Unmarshal([]byte(printed.Stdout), &report)
	if err != nil || report.Store == nil {
		t.Fatalf("doctor --json has no store field (%v):\n%s", err, printed.Stdout)
	}

	return *report.Store
}

func TestDoctorNamesTheStoresDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	state := t.TempDir()
	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir(), State: state}

	// Act
	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor")

	// Assert
	if got, want := fieldValue(printed.Stdout, "Store"), filepath.Join(state, "workflow"); got != want {
		t.Errorf("doctor's Store row = %q, want %q:\n%s", got, want, printed.Stdout)
	}
}

func TestDoctorSaysWhenTheStoreHasNoDirectory(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.Place{Dir: t.TempDir(), Home: ""}

	// Act
	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor")

	// Assert
	if got := fieldValue(printed.Stdout, "Store"); !strings.Contains(got, "could not determine a data directory") {
		t.Errorf("doctor's Store row = %q, want it to say no data directory could be determined:\n%s",
			got, printed.Stdout)
	}
}

func TestDoctorSaysWhenTheConfigurationTurnedTheStoreOff(t *testing.T) {
	t.Parallel()

	// Arrange
	where := clitest.Place{Dir: t.TempDir(), Home: t.TempDir()}
	clitest.WriteFile(t, where.Dir, `{"store": {"disabled": true}}`)

	// Act
	printed, _ := clitest.RunStreamsAt(t, where, clitest.UnusedPrompt(t), "doctor")

	// Assert
	if got := fieldValue(printed.Stdout, "Store"); got != "off (store.disabled)" {
		t.Errorf("doctor's Store row = %q, want off (store.disabled):\n%s", got, printed.Stdout)
	}
}

func TestDoctorJSONReportsTheStore(t *testing.T) {
	t.Parallel()

	state := t.TempDir()
	cases := map[string]struct {
		where         clitest.Place
		configuration string
		want          storeFacts
	}{
		"a directory": {
			where: clitest.Place{Home: t.TempDir(), State: state},
			want:  storeFacts{Dir: filepath.Join(state, "workflow")},
		},
		"no directory": {
			want: storeFacts{Problem: "could not determine a data directory for the store"},
		},
		"turned off": {
			where:         clitest.Place{Home: t.TempDir(), State: state},
			configuration: `{"store": {"disabled": true}}`,
			want:          storeFacts{Disabled: true},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			where := tt.where
			where.Dir = t.TempDir()

			if tt.configuration != "" {
				clitest.WriteFile(t, where.Dir, tt.configuration)
			}

			// Act
			got := doctorStore(t, where)

			// Assert
			if got != tt.want {
				t.Errorf("doctor --json store = %+v, want %+v", got, tt.want)
			}
		})
	}
}
