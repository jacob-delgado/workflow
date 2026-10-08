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

// ErrChangedOnDisk is a save's refusal: the files are no longer at the
// revision the write was to be made over, so writing would lose whatever
// changed them.
var ErrChangedOnDisk = errors.New("the configuration file changed on disk")

// ErrNotARevision is ParseRevision's refusal of text no Revision writes.
var ErrNotARevision = errors.New("not a configuration file revision")

// ErrTokenNotKept is a save's refusal: the keychain did not keep the Jira
// token typed into the editor, so nothing was written.
var ErrTokenNotKept = errors.New("the keychain did not keep the Jira token typed; nothing was saved")

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
	return write(path, cfg)
}

// Create writes the configuration to a new file at path, at FileMode, and
// writes nothing when anything is already there, a link included, so a link
// planted where the file goes cannot steer its credentials elsewhere. The
// refusal wraps fs.ErrExist.
func Create(path string, cfg Config) error {
	encoded, err := encode(cfg)
	if err != nil {
		return err
	}

	return createPrivate(path, encoded)
}

// write encodes the configuration into path.
func write(path string, cfg Config) error {
	encoded, err := encode(cfg)
	if err != nil {
		return err
	}

	err = writePrivate(path, encoded)
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// encode is the configuration as a file holds it.
func encode(cfg Config) ([]byte, error) {
	return indented(cfg)
}

// indented is value as JSON a person can edit, one setting to a line.
func indented(value any) ([]byte, error) {
	// Trade-off TRADE-13: a Config, and values decoded from JSON, always encode.
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encoding configuration: %w", err)
	}

	return append(encoded, '\n'), nil
}

// writePrivate replaces the file at path with one only its owner can reach,
// holding contents. The new file is written in full beside the old one and
// renamed over it, so a save that fails part way leaves the previous file as it
// was rather than empty or cut short. An empty path names no file and is
// refused before anything is written, as a read takes it for no file.
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

// createPrivate makes the file at path, which must not exist — not even as a
// link, which the exclusive create does not follow — holding contents and
// reachable only by its owner. A write that fails part way removes the file
// it made.
func createPrivate(path string, contents []byte) error {
	//nolint:gosec // the path is the user's own config file, by design
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, FileMode)
	if err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}

	err = fill(file, contents)
	if err != nil {
		return errors.Join(fmt.Errorf("writing %s: %w", path, err), os.Remove(path))
	}

	return nil
}

// followLinks is the file path names once every symbolic link on the way is
// followed, so a linked configuration file is replaced where it lives and the
// link is kept. A link to a file not yet written is followed to where that file
// will be; a path that names neither a file nor a link is path itself, in its
// directory with that directory's links resolved.
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

// followDanglingLink follows path, which names no file yet: a link to a file
// not yet written, to the end of its chain, or else path itself. It ends:
// EvalSymlinks walked this same chain and found a missing name rather than a
// loop, which it reports as an error of its own. A link the system will not
// read, past as many links as it follows in one path, is refused.
func followDanglingLink(path string) (string, error) {
	destination, err := os.Readlink(path)
	if errors.Is(err, fs.ErrNotExist) {
		return inResolvedDirectory(path)
	}

	if err != nil {
		return "", fmt.Errorf("following its links: %w", err)
	}

	return followLinks(besideLink(path, destination))
}

// besideLink is destination, read from the link at path, as a path: a
// relative one is taken from the link's own directory, appended rather than
// joined, since joining cleans each ".." against the text before it, where
// the system first follows any link in that text.
func besideLink(path, destination string) string {
	if filepath.IsAbs(destination) {
		return destination
	}

	dir, _ := filepath.Split(path)

	return dir + destination
}

// inResolvedDirectory is path in its directory with that directory's links
// resolved, so the file made beside it lands where path does. Split rather
// than Dir keeps a ".." for the resolution to take after the links before it,
// and the working directory, which a bare name is in, resolves as ".".
func inResolvedDirectory(path string) (string, error) {
	dir, base := filepath.Split(path)

	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", fmt.Errorf("following its links: %w", err)
	}

	return filepath.Join(resolved, base), nil
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

// Edit is a configuration changed in an editor — the web's Settings, or the
// terminal's — over a read of it, with each credential the editor was shown
// masked.
type Edit struct {
	// Files are the configuration files the read was made from.
	Files Files
	// Read is the configuration the editor was seeded from, unmasked.
	Read Config
	// Over is the revision of the files the read found.
	Over Revision
	// Edited is what the editor holds: a credential left masked or empty
	// stands for Read's.
	Edited Config
	// Removed are the credentials the editor removed: each is written empty,
	// whatever Edited holds for it.
	Removed []Credential
	// PlaceSlackCredentials keeps a Slack user token's secrets, typed into the
	// editor, where the configuration keeps them, and answers the configuration
	// to write. Nil keeps them in the file.
	PlaceSlackCredentials func(Config) (Config, error)
	// KeepJiraToken keeps a Jira token typed into the editor in the OS
	// keychain, under the item service names, so the file written reads it
	// from there. Nil, where no keychain is wired, keeps it in the file.
	KeepJiraToken func(service, secret string) error
}

// SaveEdit writes an edited configuration over the read it was made from,
// keeping each credential the editor left masked, and returns what it wrote
// with the revision it left. Files changed since the read are refused with
// ErrChangedOnDisk, and a credential the files read did not hold that would
// land in a repository's file with ErrCredentialInRepository; either way
// nothing is written. A Jira token typed is kept in the keychain item for its
// address where KeepJiraToken is wired, and so lands in no file. Every
// credential but those placed elsewhere is checked before any is placed,
// since placing the Slack secrets spends the refresh token typed, and every
// one before the keychain is handed the token, which replaces the item it
// held. Neither is done for a write the files would then refuse.
func SaveEdit(edit Edit) (Config, Revision, error) {
	incoming := KeepStored(edit.Edited, edit.Read, edit.Removed)
	incoming.Path, incoming.Files = edit.Files.Target(), edit.Files

	err := edit.refuseTyped(incoming, edit.heldSecretFields(incoming))
	if err != nil {
		return Config{}, Revision{}, err
	}

	incoming, err = edit.placeSlackCredentials(incoming)
	if err != nil {
		return Config{}, Revision{}, err
	}

	err = edit.refuseTyped(incoming, slackSecretFields())
	if err != nil {
		return Config{}, Revision{}, err
	}

	incoming, err = edit.keepJiraToken(incoming)
	if err != nil {
		return Config{}, Revision{}, err
	}

	written, err := SaveLayers(edit.Files, incoming, edit.Over)
	if err != nil {
		return Config{}, Revision{}, err
	}

	return incoming, written, nil
}

// placeSlackCredentials hands Slack user-token secrets that differ from the
// ones read — typed into the editor, not sent back as they were shown — to be
// kept where the configuration keeps them, and answers the configuration to
// write. A file that already keeps them goes on keeping them, with no access
// token, so the next post refreshes with what was typed.
func (edit Edit) placeSlackCredentials(incoming Config) (Config, error) {
	typed := incoming.Messaging.ClientSecret != edit.Read.Messaging.ClientSecret ||
		incoming.Messaging.RefreshToken != edit.Read.Messaging.RefreshToken
	if !typed {
		return incoming, nil
	}

	if edit.PlaceSlackCredentials == nil || edit.Read.Messaging.HoldsUserTokenSecrets() {
		incoming.Messaging.AccessToken, incoming.Messaging.ExpiresAt = "", ""

		return incoming, nil
	}

	// Placing spends the typed refresh token, so it is done only for a write
	// that will be made.
	err := edit.writableFor("placing the Slack secrets", incoming)
	if err != nil {
		return Config{}, err
	}

	return edit.PlaceSlackCredentials(incoming)
}

// heldSecretFields are the credentials incoming may hold in the file: every
// one but a Jira token typed that the keychain will keep.
func (edit Edit) heldSecretFields(incoming Config) []secretField {
	fields := fileSecretFields(incoming)
	if !edit.keepsJiraToken(incoming) {
		fields = append(fields, jiraTokenField())
	}

	return fields
}

// keepsJiraToken reports a Jira token typed into the editor — not sent back as
// it was shown — for an address, where a keychain can keep it.
func (edit Edit) keepsJiraToken(incoming Config) bool {
	typed := incoming.Jira.Token != "" && incoming.Jira.Token != edit.Read.Jira.Token

	return typed && incoming.Jira.BaseURL != "" && edit.KeepJiraToken != nil
}

// keepJiraToken keeps a Jira token typed into the editor in the keychain item
// for its address, and answers the configuration to write: reading it from
// there, the token out of the file. It is done only for a write that will be
// made, so a refused save leaves the keychain as it was.
func (edit Edit) keepJiraToken(incoming Config) (Config, error) {
	if !edit.keepsJiraToken(incoming) {
		return incoming, nil
	}

	written := incoming
	written.Jira.Token, written.Jira.Keychain = "", true

	err := edit.writableFor("keeping the Jira token", written)
	if err != nil {
		return Config{}, err
	}

	err = edit.KeepJiraToken(incoming.Jira.KeychainService(), incoming.Jira.Token.Reveal())
	if err != nil {
		return Config{}, fmt.Errorf("%w: %w", ErrTokenNotKept, err)
	}

	return written, nil
}

// writableFor refuses, writing nothing, a write of cfg that SaveLayers would
// refuse: files no longer at the revision the edit was made over, refused as
// doing, or a repository's file cfg would make a setting only the home file
// may, refused as the write would be.
func (edit Edit) writableFor(doing string, cfg Config) error {
	home, repo, err := readLayers(edit.Files)
	if err != nil || layersRevision(home, repo) != edit.Over {
		return fmt.Errorf("%s: %w", doing, ErrChangedOnDisk)
	}

	_, err = layerContents(edit.Files, home, cfg)

	return err
}
