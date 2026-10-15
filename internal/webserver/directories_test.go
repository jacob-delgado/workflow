// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/webserver/webtest"
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
	recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/directories?path=~%2Fsrc&prefix=wo")

	// Assert
	got := webtest.Decode[api.DirectoryListing](t, recorder)
	if recorder.Code != http.StatusOK || asked != "/home/ana/src wo" ||
		len(got.Entries) != 1 || got.Entries[0].Name != "worker" {
		t.Errorf("asked %q, listing %+v; want ~/src asked for wo and what it answered", asked, got)
	}
}

func TestAListingSaysTheLimitItWasCutShortAt(t *testing.T) {
	t.Parallel()

	// want is truncated_to as the body writes it, and empty where it is left
	// out.
	cases := map[string]struct {
		truncated bool
		want      string
	}{
		"a listing cut short": {truncated: true, want: strconv.Itoa(workdirs.ListLimit)},
		"a whole listing":     {truncated: false, want: ""},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			deps := repositoryDeps(favoritesKept())
			deps.Repositories.Subdirectories = func(string, string) (workdirs.Listing, error) {
				return workdirs.Listing{Entries: []workdirs.Entry{{Name: "big", Repository: false}}, Truncated: tt.truncated}, nil
			}

			// Act
			recorder := webtest.Get(t, webtest.Serve(t, deps, config.Default()), "/api/directories?path=~%2Fsrc")

			// Assert
			got := webtest.Decode[map[string]json.RawMessage](t, recorder)
			if _, old := got["truncated"]; old || string(got["truncated_to"]) != tt.want {
				t.Errorf("listing %s, want truncated_to %q and no truncated", recorder.Body, tt.want)
			}
		})
	}
}
