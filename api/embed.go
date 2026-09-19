// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package apispec embeds the OpenAPI contract so the binary carries its own
// copy of api/openapi.yaml — the same document the Go types and the TypeScript
// client are generated from. internal/webserver parses and validates it to
// check every request against the spec at runtime.
package apispec

import _ "embed"

// Spec is the raw OpenAPI document (api/openapi.yaml).
//
//go:embed openapi.yaml
var Spec []byte
