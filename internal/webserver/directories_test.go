// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"net/http"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/workdirs"
)

func TestAListingIsNarrowedToTheNamesStartingWithAPrefix(t *testing.T) {
	t.Parallel()

	// Arrange
	deps := repositoryDeps(favoritesKept())
	asked := ""
	deps.Repositories.Subdirectories = func(dir, prefix string) (workdirs.Listing, error) {
		asked = dir + " " + prefix

		return workdirs.Listing{Entries: []workdirs.Entry{{Name: "worker", Repository: true}}, Truncated: false}, nil
	}

	// Act
	recorder := get(t, serve(t, deps, config.Default()), "/api/directories?path=~%2Fsrc&prefix=wo")

	// Assert
	got := decode[api.DirectoryListing](t, recorder)
	if recorder.Code != http.StatusOK || asked != "/home/ana/src wo" ||
		len(got.Entries) != 1 || got.Entries[0].Name != "worker" {
		t.Errorf("asked %q, listing %+v; want ~/src asked for wo and what it answered", asked, got)
	}
}
