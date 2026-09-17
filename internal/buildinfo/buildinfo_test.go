// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package buildinfo_test

import (
	"runtime/debug"
	"testing"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
)

// devel is the module version the toolchain records for a source build, and
// vcsRevision the build-setting key that carries the commit.
const (
	devel       = "(devel)"
	vcsRevision = "vcs.revision"
)

func TestVersionPrefersAReleaseThenTheCommit(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		"a released module version": {
			info: &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0"}},
			ok:   true,
			want: "v0.2.0",
		},
		"built from source falls back to the short commit": {
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: devel},
				Settings: []debug.BuildSetting{{Key: vcsRevision, Value: "1a2b3c4d5e6f7a8b9c0d"}},
			},
			ok:   true,
			want: "1a2b3c4d5e6f",
		},
		// A local build stamps a synthesized pseudo-version, not an official tag,
		// so it falls back to the commit the user asked for.
		"a pseudo-version falls back to the short commit": {
			info: &debug.BuildInfo{
				Main:     debug.Module{Version: "v0.0.0-20260917173147-ddbb935d6c04"},
				Settings: []debug.BuildSetting{{Key: vcsRevision, Value: "ddbb935d6c04a1b2c3d4"}},
			},
			ok:   true,
			want: "ddbb935d6c04",
		},
		"a dirty tree is marked": {
			info: &debug.BuildInfo{
				Main: debug.Module{Version: devel},
				Settings: []debug.BuildSetting{
					{Key: vcsRevision, Value: "1a2b3c4d5e6f7a8b9c0d"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			ok:   true,
			want: "1a2b3c4d5e6f-dirty",
		},
		"no version and no revision is unknown": {
			info: &debug.BuildInfo{Main: debug.Module{Version: devel}},
			ok:   true,
			want: "unknown",
		},
		"no build info at all is unknown": {info: nil, ok: false, want: "unknown"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := buildinfo.Version(tt.info, tt.ok)

			// Assert
			if got != tt.want {
				t.Errorf("Version = %q, want %q", got, tt.want)
			}
		})
	}
}
