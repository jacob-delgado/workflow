// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import "fmt"

// validateVersion refuses a file that names a version this build does not know.
// An empty version is the current one, so an unversioned file still loads.
func (c Config) validateVersion() error {
	if c.Version != "" && c.Version != CurrentVersion {
		return fmt.Errorf("%w: %q; this build reads version %s", ErrUnknownVersion, c.Version, CurrentVersion)
	}

	return nil
}
