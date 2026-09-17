// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestUISettingsKeepTheirDefaultsUnlessAValidFileSetsThem(t *testing.T) {
	t.Parallel()

	defaults := config.UI{Mouse: true, ASCII: false}

	cases := map[string]struct {
		contents string
		want     config.UI
		wantErr  error
	}{
		// Every configuration written before these settings existed has no ui
		// block. It must keep the mouse, not silently lose it.
		"no ui block":      {contents: completeConfig, want: defaults},
		"mouse turned off": {contents: `{"ui": {"mouse": false}}`, want: config.UI{Mouse: false, ASCII: false}},
		// Naming one setting must not reset the other to its zero value.
		"only ascii named": {contents: `{"ui": {"ascii": true}}`, want: config.UI{Mouse: true, ASCII: true}},
		// A file that does not load still leaves the interface its defaults, so
		// it opens with the mouse working to say what is wrong.
		"a setting of the wrong type": {contents: `{"ui": {"mouse": "yes"}}`, want: defaults, wantErr: config.ErrInvalid},
		"a misspelled setting":        {contents: `{"ui": {"mice": false}}`, want: defaults, wantErr: config.ErrInvalid},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path := write(t, t.TempDir(), tt.contents)

			// Act
			cfg, err := config.LoadFile(path)

			// Assert
			if !errors.Is(err, tt.wantErr) || cfg.UI != tt.want {
				t.Errorf("LoadFile = %+v, %v; want %+v, %v", cfg.UI, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestAConfigurationThatWasNotFoundStillHasTheDefaults(t *testing.T) {
	t.Parallel()

	// Act
	cfg, err := config.Load(t.TempDir(), t.TempDir())

	// Assert
	// The interface still opens without a configuration, to say what is wrong,
	// and it should open with the mouse working.
	if !errors.Is(err, config.ErrNotFound) || !cfg.UI.Mouse {
		t.Errorf("Load = %+v, %v; want the defaults and ErrNotFound", cfg.UI, err)
	}
}

func TestTheTemplateWritesTheUISettings(t *testing.T) {
	t.Parallel()

	// Act & Assert
	// config init is how people discover a setting exists at all.
	if got := config.Template().UI; got != (config.UI{Mouse: true, ASCII: false}) {
		t.Errorf("Template().UI = %+v, want the defaults written out", got)
	}
}
