// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
)

// ErrChangedOnDisk is SaveOver's refusal: the file is no longer at the revision
// the write was to be made over, so writing would lose whatever changed it.
var ErrChangedOnDisk = errors.New("the configuration file changed on disk")

// ErrNotARevision is ParseRevision's refusal of text no Revision writes.
var ErrNotARevision = errors.New("not a configuration file revision")

// Revision is one state of the configuration file: a keyed SHA-256 (HMAC) of
// its bytes, or, as the zero value, no file at all. Two revisions are the same
// state exactly when they are ==. A hash of the contents rather than a
// modification time, since an edit that keeps the size inside one clock tick
// still changes the hash, and a touch that changes nothing does not.
type Revision struct {
	digest [sha256.Size]byte
	exists bool
}

// noFile is the text form of the revision of a file that does not exist.
const noFile = "none"

// Exists reports whether the revision is of a file, rather than of its absence.
func (r Revision) Exists() bool {
	return r.exists
}

// String is the revision's text form, which ParseRevision reads back: the
// digest in hexadecimal, or "none" when there is no file.
func (r Revision) String() string {
	if !r.exists {
		return noFile
	}

	return hex.EncodeToString(r.digest[:])
}

// ParseRevision reads a revision back from its String form.
func ParseRevision(text string) (Revision, error) {
	if text == noFile {
		return Revision{}, nil
	}

	digest, err := hex.DecodeString(text)
	if err != nil || len(digest) != sha256.Size {
		return Revision{}, ErrNotARevision
	}

	return Revision{digest: [sha256.Size]byte(digest), exists: true}, nil
}

// RevisionOf reads the revision of the file at path. A file that does not
// exist, or an empty path, which names none, is the no-file revision rather
// than an error.
func RevisionOf(path string) (Revision, error) {
	//nolint:gosec // the path is the user's own config file, by design
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Revision{}, nil
	}

	if err != nil {
		return Revision{}, fmt.Errorf("reading %s: %w", path, err)
	}

	return revisionOfContents(contents), nil
}

// revisionKey keys each revision's digest. A revision leaves the process as
// the web API's ETag, and the file holds credentials, so the digest is keyed:
// it tells whoever holds it whether the file changed, and nothing about what
// the file says. Drawn once per process, it keeps revisions comparable with ==
// for as long as the server handing them out runs; after a restart a Settings
// form still open names a revision no file has, so its save is refused and
// Reload answers it.
//
//nolint:gochecknoglobals // one key per process is the point, and no caller holds it
var revisionKey = []byte(rand.Text())

// revisionOfContents is the revision of a file holding contents.
func revisionOfContents(contents []byte) Revision {
	mac := hmac.New(sha256.New, revisionKey)
	mac.Write(contents)

	return Revision{digest: [sha256.Size]byte(mac.Sum(nil)), exists: true}
}

// Save writes the configuration to path at FileMode, over whatever is there.
func Save(path string, cfg Config) error {
	_, err := write(path, cfg)

	return err
}

// SaveOver writes the configuration to path as Save does, but only while the
// file is still at the revision over; otherwise it writes nothing and returns
// ErrChangedOnDisk. It returns the revision of what it wrote. The check and the
// write are not one step, so another process can still slip a write between
// them; closing that would take a lock every writer honors, and an editor
// honors none.
func SaveOver(path string, cfg Config, over Revision) (Revision, error) {
	current, err := RevisionOf(path)
	if err != nil {
		return Revision{}, err
	}

	if current != over {
		return Revision{}, fmt.Errorf("%s: %w", path, ErrChangedOnDisk)
	}

	contents, err := write(path, cfg)
	if err != nil {
		return Revision{}, err
	}

	return revisionOfContents(contents), nil
}

// write encodes the configuration into path and returns the bytes it wrote.
func write(path string, cfg Config) ([]byte, error) {
	encoded, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding configuration: %w", err)
	}

	encoded = append(encoded, '\n')

	err = writePrivate(path, encoded)
	if err != nil {
		return nil, fmt.Errorf("writing %s: %w", path, err)
	}

	return encoded, nil
}

// writePrivate writes a file only its owner can reach. Asking for FileMode when
// opening it is not enough: that is honored for a file being created, and one
// that already exists keeps the mode it had. So the mode is set on the open
// file, before anything is written into it.
func writePrivate(path string, contents []byte) error {
	//nolint:gosec // the path is the user's own config file, by design
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, FileMode)
	if err != nil {
		return fmt.Errorf("opening it: %w", err)
	}

	err = file.Chmod(FileMode)
	if err == nil {
		_, err = file.Write(contents)
	}

	return errors.Join(err, file.Close())
}

// othersMask is the permission bits that belong to anyone but a file's owner.
const othersMask os.FileMode = 0o077

// SharedMode reports the permissions of a file that someone other than its owner
// can read or write, which a file holding credentials must not be.
func SharedMode(path string) (os.FileMode, bool) {
	// Windows keeps no such bits, and Go reports 0666 for every file there,
	// which says nothing about who can read it.
	if runtime.GOOS == "windows" {
		return 0, false
	}

	info, err := os.Stat(path)
	if err != nil {
		return 0, false
	}

	mode := info.Mode().Perm()

	return mode, mode&othersMask != 0
}
