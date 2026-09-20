// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package web_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/web"
)

func TestAssetsReportsNoEmbeddedUIInTheDefaultBuild(t *testing.T) {
	t.Parallel()

	// Act
	assets, ok := web.Assets()

	// Assert
	if ok || assets != nil {
		t.Errorf("Assets() = (%v, %t), want (nil, false) without the embedui build tag", assets, ok)
	}
}
