// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"errors"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
)

func TestUISettingsDefaultWhenTheFileLeavesThemOut(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		contents string
		want     config.UI
	}{
		// Every configuration written before these settings existed has no ui
		// block. It must keep the mouse, not silently lose it.
		"no ui block":      {contents: completeConfig, want: config.UI{Mouse: true, ASCII: false}},
		"mouse turned off": {contents: `{"ui": {"mouse": false}}`, want: config.UI{Mouse: false, ASCII: false}},
		// Naming one setting must not reset the other to its zero value.
		"only ascii named": {contents: `{"ui": {"ascii": true}}`, want: config.UI{Mouse: true, ASCII: true}},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cfg, err := config.LoadFile(write(t, t.TempDir(), tt.contents))
			if err != nil {
				t.Fatalf("LoadFile returned %v, want nil", err)
			}

			if cfg.UI != tt.want {
				t.Errorf("UI = %+v, want %+v", cfg.UI, tt.want)
			}
		})
	}
}

func TestAConfigurationThatDidNotLoadStillHasTheDefaults(t *testing.T) {
	t.Parallel()

	// The interface still opens without a configuration, to say what is wrong,
	// and it should open with the mouse working.
	cfg, err := config.Load(t.TempDir(), t.TempDir())
	if !errors.Is(err, config.ErrNotFound) {
		t.Fatalf("Load returned %v, want ErrNotFound", err)
	}

	if !cfg.UI.Mouse {
		t.Error("UI.Mouse = false for a configuration that did not load, want the default")
	}

	cfg, err = config.LoadFile(write(t, t.TempDir(), `{"ui": {"mouse": "yes"}}`))
	if !errors.Is(err, config.ErrInvalid) || !cfg.UI.Mouse {
		t.Errorf("LoadFile returned %+v, %v, want the defaults and ErrInvalid", cfg.UI, err)
	}
}

func TestUnknownUISettingsAreRejected(t *testing.T) {
	t.Parallel()

	_, err := config.LoadFile(write(t, t.TempDir(), `{"ui": {"mice": false}}`))
	if !errors.Is(err, config.ErrInvalid) {
		t.Errorf("LoadFile returned %v, want ErrInvalid for a misspelled setting", err)
	}
}

func TestTheTemplateWritesTheUISettings(t *testing.T) {
	t.Parallel()

	// config init is how people discover a setting exists at all.
	if got := config.Template().UI; got != (config.UI{Mouse: true, ASCII: false}) {
		t.Errorf("Template().UI = %+v, want the defaults written out", got)
	}
}
