// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

// These tests reach the keyMap and the help rendering directly, which the
// external package cannot, to prove the help is complete by construction. They
// live in package tui — testpackage's default skip covers *internal_test.go —
// because the reflection they need does not cross the package boundary, which is
// where gobco's instrumenter fails.

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"

	"github.com/jacob-delgado/workflow/internal/config"
)

// helpModel is a model whose help can be rendered without the world fixture.
func helpModel() Model {
	return New(config.Config{}, nil, Deps{})
}

func TestHelpListsEveryBinding(t *testing.T) {
	t.Parallel()

	// Arrange
	model := helpModel()
	help := model.helpView()

	// Act & Assert
	for _, binding := range model.keys.everyBinding() {
		named := binding.Help()
		if named.Key == "" {
			continue
		}

		if !strings.Contains(help, named.Key) || !strings.Contains(help, named.Desc) {
			t.Errorf("the help omits %q (%s):\n%s", named.Desc, named.Key, help)
		}
	}
}

func TestEveryKeyMapFieldIsRegisteredForHelp(t *testing.T) {
	t.Parallel()

	// Arrange
	//nolint:modernize // reflect.TypeFor with the Fields iterator crashes gobco.
	keyMapType, bindingType := reflect.TypeOf(keyMap{}), reflect.TypeOf(key.Binding{})

	fields := 0

	//nolint:intrange,modernize // an integer range and the Fields iterator crash gobco.
	for index := 0; index < keyMapType.NumField(); index++ {
		if keyMapType.Field(index).Type == bindingType {
			fields++
		}
	}

	// Act & Assert
	if got := len(helpModel().keys.everyBinding()); got != fields {
		t.Errorf("everyBinding lists %d bindings, keyMap has %d fields", got, fields)
	}
}
