// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import "strings"

// RememberScope hands record the scope a commit just used, trimmed as the
// subject line writes it, and reports whether it did, with why record could
// not keep it. A blank scope is not recorded: it would erase the scope learned
// before and hide the configured default the next commit opens on. Nothing is
// recorded without a record to hand it to, as when the store is off.
func RememberScope(record func(scope string) error, scope string) (bool, error) {
	written := strings.TrimSpace(scope)
	if written == "" || record == nil {
		return false, nil
	}

	return true, record(written)
}
