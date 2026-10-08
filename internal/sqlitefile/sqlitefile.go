// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

// Package sqlitefile opens a SQLite database file through the pure-Go driver
// itself, so CGO stays off and no lookup of a driver by name stands between a
// caller and its file. The file is named by a file URI built from its path,
// so a path holding a character a URI gives a meaning to still names that
// file.
package sqlitefile

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"strings"

	"modernc.org/sqlite"
)

// connector opens each connection to the database dsn names with the pure-Go
// driver itself.
type connector struct {
	dsn string
}

var _ driver.Connector = connector{}

// Connect opens one connection to the database.
func (c connector) Connect(context.Context) (driver.Conn, error) {
	conn, err := (&sqlite.Driver{}).Open(c.dsn)
	if err != nil {
		return nil, fmt.Errorf("opening the database: %w", err)
	}

	return conn, nil
}

// Driver is the pure-Go SQLite driver.
func (connector) Driver() driver.Driver {
	return &sqlite.Driver{}
}

// Open is a handle on the database file at path, opened with the URI
// parameters params, which connects only once it is first used.
func Open(path, params string) *sql.DB {
	return sql.OpenDB(connector{dsn: fileURI(path, params)})
}

// fileURI is the address of the database file at path with params: a file
// URI, in which a percent sign escapes, a question mark starts the parameters
// and a hash ends the path, so the path escapes all three.
func fileURI(path, params string) string {
	escaped := strings.NewReplacer("%", "%25", "?", "%3F", "#", "%23").Replace(filepath.ToSlash(path))

	return "file:" + escaped + "?" + params
}
