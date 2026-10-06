// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// storeFacts is the store as doctor --json reports it.
type storeFacts struct {
	Dir      string `json:"dir"`
	Disabled bool   `json:"disabled"`
	Problem  string `json:"problem"`
}

// doctorStore runs doctor --json at where and returns its store field.
func doctorStore(t *testing.T, where place) storeFacts {
	t.Helper()

	printed, _ := runStreamsAt(t, where, unusedPrompt(t), "doctor", "--json")

	var report struct {
		Store *storeFacts `json:"store"`
	}

	err := json.Unmarshal([]byte(printed.stdout), &report)
	if err != nil || report.Store == nil {
		t.Fatalf("doctor --json has no store field (%v):\n%s", err, printed.stdout)
	}

	return *report.Store
}

func TestDoctorNamesTheStoresDirectory(t *testing.T) {
	// Arrange
	state := t.TempDir()
	where := place{dir: t.TempDir(), home: t.TempDir(), state: state}

	// Act
	printed, _ := runStreamsAt(t, where, unusedPrompt(t), "doctor")

	// Assert
	if got, want := fieldValue(printed.stdout, "Store"), filepath.Join(state, "workflow"); got != want {
		t.Errorf("doctor's Store row = %q, want %q:\n%s", got, want, printed.stdout)
	}
}

func TestDoctorSaysWhenTheStoreHasNoDirectory(t *testing.T) {
	// Arrange
	where := place{dir: t.TempDir(), home: ""}

	// Act
	printed, _ := runStreamsAt(t, where, unusedPrompt(t), "doctor")

	// Assert
	if got := fieldValue(printed.stdout, "Store"); !strings.Contains(got, "could not determine a data directory") {
		t.Errorf("doctor's Store row = %q, want it to say no data directory could be determined:\n%s",
			got, printed.stdout)
	}
}

func TestDoctorSaysWhenTheConfigurationTurnedTheStoreOff(t *testing.T) {
	// Arrange
	where := place{dir: t.TempDir(), home: t.TempDir()}
	writeFile(t, where.dir, `{"store": {"disabled": true}}`)

	// Act
	printed, _ := runStreamsAt(t, where, unusedPrompt(t), "doctor")

	// Assert
	if got := fieldValue(printed.stdout, "Store"); got != "off (store.disabled)" {
		t.Errorf("doctor's Store row = %q, want off (store.disabled):\n%s", got, printed.stdout)
	}
}

func TestDoctorJSONReportsTheStore(t *testing.T) {
	state := t.TempDir()
	cases := map[string]struct {
		where         place
		configuration string
		want          storeFacts
	}{
		"a directory": {
			where: place{home: t.TempDir(), state: state},
			want:  storeFacts{Dir: filepath.Join(state, "workflow")},
		},
		"no directory": {
			want: storeFacts{Problem: "could not determine a data directory for the store"},
		},
		"turned off": {
			where:         place{home: t.TempDir(), state: state},
			configuration: `{"store": {"disabled": true}}`,
			want:          storeFacts{Disabled: true},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			where := tt.where
			where.dir = t.TempDir()

			if tt.configuration != "" {
				writeFile(t, where.dir, tt.configuration)
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
