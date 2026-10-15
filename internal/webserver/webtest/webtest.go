// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package webtest is what the tests of internal/webserver and of the packages
// that leave it share: the harness that serves the API handler under a session
// and sends it requests, the canned seams and fixture constants, the event
// stream's reader, a seam call held on its context, the configuration's edits,
// the Taskwarrior fake and lefthook's fixtures. Each test still drives its
// code black-box: the server through webserver.Handler, or a package that left
// through its exported API. Only tests import it.
package webtest
