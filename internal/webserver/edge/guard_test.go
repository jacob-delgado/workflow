// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package edge_test

import (
	"net/http"
	"strings"
	"testing"
)

func TestNonLoopbackHostsAreRejected(t *testing.T) {
	t.Parallel()

	// A foreign name (a rebound hostname) and a foreign IP literal both name
	// somewhere other than the loopback interface.
	for _, host := range []string{"attacker.example.com", "attacker.example.com:7000", "10.0.0.1:7000"} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ops := &operations{}
			handler := serveOver(t, ops)

			// Act
			recorder := call{method: http.MethodGet, target: healthPath, host: host}.send(t, handler)

			// Assert
			if recorder.Code != http.StatusForbidden {
				t.Errorf("status = %d for host %q, want 403 — a non-loopback host must be refused", recorder.Code, host)
			}

			if ops.reached.Load() != 0 {
				t.Error("the refused request reached the server")
			}
		})
	}
}

func TestLoopbackHostsReachTheAPI(t *testing.T) {
	t.Parallel()

	// The loopback name, in either case, and IP literals, with and without a
	// port.
	for _, host := range []string{"127.0.0.1:13579", "localhost:13579", "LOCALHOST", "[::1]:13579", "127.0.0.1"} {
		t.Run(host, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ops := &operations{}
			handler := serveOver(t, ops)

			// Act
			recorder := call{method: http.MethodGet, target: healthPath, host: host}.send(t, handler)

			// Assert
			if recorder.Code != http.StatusOK || ops.reached.Load() != 1 {
				t.Errorf("status = %d and reached %d times for host %q, want 200 once — a loopback host must reach the API",
					recorder.Code, ops.reached.Load(), host)
			}
		})
	}
}

func TestWritesAreRefusedInDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}
	handler := serveReadOnly(t, ops)

	// Act
	// A same-origin write that would otherwise be allowed.
	recorder := checkout("").send(t, handler)

	// Assert
	if recorder.Code != http.StatusForbidden || ops.reached.Load() != 0 {
		t.Errorf("status = %d and reached %d times, want 403 and none — the web interface is read-only in dry-run",
			recorder.Code, ops.reached.Load())
	}
}

func TestReadsAreServedInDryRun(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}
	handler := serveReadOnly(t, ops)

	// Act
	recorder := get(t, handler, branchPath)

	// Assert
	if recorder.Code != http.StatusOK || ops.reached.Load() != 1 {
		t.Errorf("status = %d and reached %d times, want 200 once — dry-run refuses writes alone",
			recorder.Code, ops.reached.Load())
	}
}

func TestACrossOriginWriteIsRefused(t *testing.T) {
	t.Parallel()

	// A foreign host, and a page on another loopback port — both cross-origin to
	// the server's own 127.0.0.1:13579, so both must be refused.
	for _, origin := range []string{"http://evil.example.com", "http://127.0.0.1:3000", "http://localhost:5173"} {
		t.Run(origin, func(t *testing.T) {
			t.Parallel()

			// Arrange
			ops := &operations{}
			handler := serveOver(t, ops)

			// Act
			recorder := checkout(origin).send(t, handler)

			// Assert
			if recorder.Code != http.StatusForbidden || ops.reached.Load() != 0 {
				t.Errorf("status = %d and reached %d times for origin %q, want 403 and none — a cross-origin write must be refused",
					recorder.Code, ops.reached.Load(), origin)
			}
		})
	}
}

func TestAMalformedOriginIsRefusedForAWrite(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serveOver(t, &operations{})

	// Act
	recorder := checkout("http://%zz").send(t, handler)

	// Assert
	if recorder.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a write whose origin does not parse", recorder.Code)
	}
}

func TestASafeMethodSkipsTheWriteGuard(t *testing.T) {
	t.Parallel()

	// Arrange
	handler := serveOver(t, &operations{})

	// Act
	recorder := call{method: http.MethodOptions, target: checkoutPath, origin: "http://evil.example.com"}.send(t, handler)

	// Assert
	// A safe method is exempt from the cross-origin write guard, so even a foreign
	// origin passes it, to the validator, which answers that the checkout takes
	// no OPTIONS.
	if recorder.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want the validator's 405 — a safe method must pass the cross-origin write guard",
			recorder.Code)
	}
}

func TestASameOriginWriteIsNotRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}
	handler := serveOver(t, ops)

	// Act
	recorder := checkout("http://127.0.0.1:13579").send(t, handler)

	// Assert
	if recorder.Code != http.StatusOK || ops.reached.Load() != 1 {
		t.Errorf("status = %d and reached %d times, want the guard to pass a same-origin write to the server",
			recorder.Code, ops.reached.Load())
	}
}

func TestAWriteWithNoOriginIsNotRefused(t *testing.T) {
	t.Parallel()

	// Arrange
	ops := &operations{}
	handler := serveOver(t, ops)

	// Act
	// curl and the like send no Origin; only a browser's cross-origin write does.
	recorder := checkout("").send(t, handler)

	// Assert
	if recorder.Code != http.StatusOK || ops.reached.Load() != 1 {
		t.Errorf("status = %d and reached %d times, want the guard to pass a write with no origin to the server",
			recorder.Code, ops.reached.Load())
	}
}

func TestEveryAnswerRefusesToBeFramedOrSniffed(t *testing.T) {
	t.Parallel()

	cases := map[string]struct{ host, target string }{
		"the app's root":       {host: loopbackHost, target: "/"},
		"the app's index":      {host: loopbackHost, target: "/index.html"},
		"an asset":             {host: loopbackHost, target: "/assets/app.js"},
		"the API":              {host: loopbackHost, target: healthPath},
		"a refused host's ask": {host: "evil.example:13579", target: "/"},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			recorder := call{method: http.MethodGet, target: tt.target, host: tt.host}.send(t, serveOver(t, &operations{}))

			// Assert
			policy := recorder.Header().Get("Content-Security-Policy")
			for _, directive := range []string{"default-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
				if !strings.Contains(policy, directive) {
					t.Errorf("Content-Security-Policy = %q, want it to hold %q", policy, directive)
				}
			}

			for header, want := range map[string]string{
				"X-Frame-Options": "DENY", "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer",
			} {
				if got := recorder.Header().Get(header); got != want {
					t.Errorf("%s = %q, want %q", header, got, want)
				}
			}
		})
	}
}
