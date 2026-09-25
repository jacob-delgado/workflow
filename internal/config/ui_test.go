// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"reflect"
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
		"color set to never": {
			contents: `{"ui": {"color": "never"}}`,
			want:     config.UI{Mouse: true, ASCII: false, Color: "never"},
		},
		// A ui.keys map is read in as the raw action-to-key overrides; the tui
		// layer validates them, config only carries them.
		"a keys map parses": {
			contents: `{"ui": {"keys": {"commit": "C", "comment": "ctrl+e"}}}`,
			want:     config.UI{Mouse: true, ASCII: false, Keys: map[string]string{"commit": "C", "comment": "ctrl+e"}},
		},
		// A file that does not load still leaves the interface its defaults, so
		// it opens with the mouse working to say what is wrong.
		"a setting of the wrong type": {contents: `{"ui": {"mouse": "yes"}}`, want: defaults, wantErr: config.ErrInvalid},
		"a misspelled setting":        {contents: `{"ui": {"mice": false}}`, want: defaults, wantErr: config.ErrInvalid},
		// The detail pane keeps the last comments_shown comments, so a negative
		// count would reach past the end of the list.
		"a negative comments_shown": {contents: `{"ui": {"comments_shown": -1}}`, want: defaults, wantErr: config.ErrInvalid},
		// Only "never" turns the hues off, so a misspelling of it would draw them.
		"a misspelled color": {contents: `{"ui": {"color": "nevr"}}`, want: defaults, wantErr: config.ErrInvalid},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			path := write(t, t.TempDir(), tt.contents)

			// Act
			cfg, err := config.LoadFile(path)

			// Assert
			if !errors.Is(err, tt.wantErr) || !reflect.DeepEqual(cfg.UI, tt.want) {
				t.Errorf("LoadFile = %+v, %v; want %+v, %v", cfg.UI, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestColorIsOffUnderNoColorOrTheSetting(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		ui      config.UI
		noColor string
		want    bool
	}{
		"default draws color": {ui: config.UI{}, noColor: "", want: true},
		"NO_COLOR set":        {ui: config.UI{}, noColor: "1", want: false},
		"ui.color never":      {ui: config.UI{Color: "never"}, noColor: "", want: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.ui.DrawColor(tt.noColor); got != tt.want {
				t.Errorf("DrawColor(%q) with %+v = %v, want %v", tt.noColor, tt.ui, got, tt.want)
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
	if got := config.Template().UI; !reflect.DeepEqual(got, config.UI{Mouse: true, ASCII: false}) {
		t.Errorf("Template().UI = %+v, want the defaults written out", got)
	}
}
