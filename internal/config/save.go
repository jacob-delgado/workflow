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
	"path/filepath"
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

// Save writes the configuration to path at FileMode, replacing whatever is
// there, or the file a link there points at. The new file is made in the
// directory of the file it replaces, so that directory must be writable too.
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

// writePrivate replaces the file at path with one only its owner can reach,
// holding contents. The new file is written in full beside the old one and
// renamed over it, so a save that fails part way leaves the previous file as it
// was rather than empty or cut short. An empty path names no file and is
// refused before anything is written, as RevisionOf reads it as no file.
func writePrivate(path string, contents []byte) error {
	if path == "" {
		return &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}

	target, err := followLinks(path)
	if err != nil {
		return err
	}

	replacement, err := writeBeside(target, contents)
	if err != nil {
		return err
	}

	err = os.Rename(replacement, target)
	if err != nil {
		return errors.Join(fmt.Errorf("putting its replacement in place: %w", err), os.Remove(replacement))
	}

	return nil
}

// followLinks is the file path names once every symbolic link on the way is
// followed, so a linked configuration file is replaced where it lives and the
// link is kept. A link to a file not yet written is followed to where that file
// will be; a path that names neither a file nor a link is path itself.
func followLinks(path string) (string, error) {
	target, err := filepath.EvalSymlinks(path)
	if errors.Is(err, fs.ErrNotExist) {
		return followDanglingLink(path)
	}

	if err != nil {
		return "", fmt.Errorf("following its links: %w", err)
	}

	return target, nil
}

// followDanglingLink follows a link to a file not yet written to the end of its
// chain; path itself when path is no link. It ends: EvalSymlinks walked this
// same chain and found a missing name rather than a loop, which it reports as
// an error of its own.
func followDanglingLink(path string) (string, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && info.Mode()&fs.ModeSymlink == 0) {
		return path, nil
	}

	if err != nil {
		return "", fmt.Errorf("following its links: %w", err)
	}

	next, err := linkDestination(path)
	if err != nil {
		return "", err
	}

	return followLinks(next)
}

// linkDestination is the path the link at path names. A relative one is taken
// from the link's own directory with that directory's links resolved, and is
// appended rather than joined: joining cleans each ".." against the text before
// it, where the system first follows any link in that text.
func linkDestination(path string) (string, error) {
	destination, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("following its links: %w", err)
	}

	if filepath.IsAbs(destination) {
		return destination, nil
	}

	dir, _ := filepath.Split(path)

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("following its links: %w", err)
	}

	return resolved + string(filepath.Separator) + destination, nil
}

// writeBeside writes contents to a new file in target's directory and returns
// its path, removing the file again when the write fails.
func writeBeside(target string, contents []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(target), "."+FileName+".*")
	if err != nil {
		return "", fmt.Errorf("creating its replacement: %w", err)
	}

	err = fill(file, contents)
	if err != nil {
		return "", errors.Join(err, os.Remove(file.Name()))
	}

	return file.Name(), nil
}

// fill writes contents into file at FileMode and closes it. CreateTemp's mode
// is whatever the umask leaves of 0600, so the mode is set rather than assumed,
// before anything is written; and the contents are flushed to the disk before
// the rename can give them the configuration file's name.
func fill(file *os.File, contents []byte) error {
	chmodErr := file.Chmod(FileMode)
	_, writeErr := file.Write(contents)

	return errors.Join(chmodErr, writeErr, file.Sync(), file.Close())
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
