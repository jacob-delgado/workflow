// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"reflect"
)

// Files are the configuration files that apply to a directory: the home
// directory's, and the repository's layered over it, setting by setting. Either
// may be empty, for no file there.
type Files struct {
	Home string
	Repo string
}

// Target is the file a save writes: the repository's when there is one, so a
// change made while working in a repository stays with it, and the home file
// stays the default every repository shares.
func (f Files) Target() string {
	if f.Repo != "" {
		return f.Repo
	}

	return f.Home
}

// String names the files, the repository's over the home directory's when
// both apply.
func (f Files) String() string {
	if f.Home != "" && f.Repo != "" {
		return f.Repo + " over " + f.Home
	}

	return f.Target()
}

// Each is every file f names, the home directory's first, as it is read.
func (f Files) Each() []string {
	var named []string

	for _, path := range []string{f.Home, f.Repo} {
		if path != "" {
			named = append(named, path)
		}
	}

	return named
}

// Layers are the files c was read from, or the one file Path names when it
// was set without them, as a configuration built in place of a read is.
func (c Config) Layers() Files {
	if c.Files == (Files{}) && c.Path != "" {
		return Files{Repo: c.Path}
	}

	return c.Files
}

// Locate finds the configuration files that apply: the nearest one walking up
// from workDir, no higher than the repository root and no higher than workDir
// outside one, layered over the one in homeDir. The home directory's file is
// the home file wherever it is found from. It returns ErrNotFound when neither
// location has one.
func Locate(workDir, homeDir string) (Files, error) {
	var files Files

	if found, ok := fileIn(homeDir); ok {
		files.Home = found
	}

	if found, ok := nearest(workDir); ok && found != files.Home {
		files.Repo = found
	}

	if files == (Files{}) {
		return Files{}, fmt.Errorf("%w in %s or %s", ErrNotFound, workDir, homeDir)
	}

	return files, nil
}

// layer is one configuration file as read: its path, and its bytes when it
// exists.
type layer struct {
	path     string
	contents []byte
	exists   bool
}

// LoadLayersAt reads the configuration files names, the repository's over the
// home directory's, with the revision of the pair. Each file's keys are checked
// on their own, so an unknown one names its file; the layers are then merged
// and validated as one, since two valid files can still disagree. With neither
// file there, it is the defaults at the no-file revision.
func LoadLayersAt(files Files) (Config, Revision, error) {
	home, repo, err := readLayers(files)
	if err != nil {
		return Default(), Revision{}, err
	}

	cfg := Default()

	if home.exists || repo.exists {
		cfg, err = parseLayers(files, home, repo)
		if err != nil {
			return Default(), Revision{}, err
		}
	}

	cfg.Path, cfg.Files = files.Target(), files

	return cfg, layersRevision(home, repo), nil
}

// RevisionOfLayers reads the revision of the configuration files names.
func RevisionOfLayers(files Files) (Revision, error) {
	home, repo, err := readLayers(files)
	if err != nil {
		return Revision{}, err
	}

	return layersRevision(home, repo), nil
}

// SaveLayers writes the configuration to the file files saves to, while the
// pair is still at the revision over; otherwise it writes nothing and returns
// ErrChangedOnDisk. Written over a home file, the repository's file holds only
// what differs from it, so a setting inherited — a token among them — is never
// copied into a file in a working tree, and a later change at home still
// reaches it. It returns the revision of the pair it leaves.
func SaveLayers(files Files, cfg Config, over Revision) (Revision, error) {
	home, repo, err := readLayers(files)
	if err != nil {
		return Revision{}, err
	}

	if layersRevision(home, repo) != over {
		return Revision{}, fmt.Errorf("%s: %w", files, ErrChangedOnDisk)
	}

	contents, err := layerContents(files, home, cfg)
	if err != nil {
		return Revision{}, err
	}

	err = writePrivate(files.Target(), contents)
	if err != nil {
		return Revision{}, fmt.Errorf("writing %s: %w", files.Target(), err)
	}

	return RevisionOfLayers(files)
}

// CreateLayers writes the configuration as SaveLayers does, but only as a new
// file: like Create, it writes nothing when anything — a link included — is
// already where the file goes.
func CreateLayers(files Files, cfg Config, over Revision) error {
	home, repo, err := readLayers(files)
	if err != nil {
		return err
	}

	if layersRevision(home, repo) != over {
		return fmt.Errorf("%s: %w", files, ErrChangedOnDisk)
	}

	contents, err := layerContents(files, home, cfg)
	if err != nil {
		return err
	}

	return createPrivate(files.Target(), contents)
}

// readLayers reads the home and repository files.
func readLayers(files Files) (layer, layer, error) {
	home, err := readLayer(files.Home)
	if err != nil {
		return layer{}, layer{}, err
	}

	repo, err := readLayer(files.Repo)

	return home, repo, err
}

// readLayer reads the file at path; a path that names none, or a file that is
// not there, is a layer that does not exist rather than an error.
func readLayer(path string) (layer, error) {
	if path == "" {
		return layer{}, nil
	}

	//nolint:gosec // the path is the user's own config file, by design
	contents, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return layer{path: path}, nil
	}

	if err != nil {
		return layer{}, fmt.Errorf("reading %s: %w", path, err)
	}

	return layer{path: path, contents: contents, exists: true}, nil
}

// parseLayers checks each file's keys, merges the files and validates the
// result.
func parseLayers(files Files, home, repo layer) (Config, error) {
	merged, err := mergeLayers(home, repo)
	if err != nil {
		return Default(), err
	}

	cfg, err := Parse(bytes.NewReader(merged))
	if err != nil {
		return Default(), fmt.Errorf("%s: %w", files, err)
	}

	return cfg, nil
}

// mergeLayers is the repository's file over the home directory's, as one
// JSON object: an object in both is merged key by key, and anything else the
// repository sets — a list, a string, an explicit false — replaces the home
// file's. A section the repository points somewhere else inherits none of the
// home file's credentials for it.
func mergeLayers(home, repo layer) ([]byte, error) {
	beneath, err := layerValue(home)
	if err != nil {
		return nil, err
	}

	over, err := layerValue(repo)
	if err != nil {
		return nil, err
	}

	moved, err := movedSections(beneath, over)
	if err != nil {
		return nil, err
	}

	return encodeValue(mergeValues(withoutCredentials(beneath, moved), over))
}

// layerValue is what file holds as a JSON value, its keys checked against the
// configuration's, or an empty object when there is no file.
func layerValue(file layer) (any, error) {
	if !file.exists {
		return map[string]any{}, nil
	}

	_, err := decode(file.contents)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file.path, err)
	}

	// Trade-off TRADE-13: decode has just read these bytes as JSON.
	return jsonValue(file.contents)
}

// encodeValue is value as JSON.
func encodeValue(value any) ([]byte, error) {
	// Trade-off TRADE-13: values decoded from JSON always encode.
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("merging configuration: %w", err)
	}

	return encoded, nil
}

// mergeValues lays over on base.
func mergeValues(base, over any) any {
	baseObject, baseIsObject := base.(map[string]any)
	overObject, overIsObject := over.(map[string]any)

	if !baseIsObject || !overIsObject {
		return over
	}

	merged := maps.Clone(baseObject)
	for key, value := range overObject {
		merged[key] = mergeValues(baseObject[key], value)
	}

	return merged
}

// layerContents is what the file files saves to holds for cfg: every setting,
// unless it lies over a home file, when it is only what differs from that. The
// home file is read unvalidated, since it need only be valid with its layer.
func layerContents(files Files, home layer, cfg Config) ([]byte, error) {
	if files.Repo == "" || !home.exists {
		return encode(cfg)
	}

	beneath, err := decode(home.contents)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", home.path, err)
	}

	full, err := configValue(cfg)
	if err != nil {
		return nil, err
	}

	base, err := configValue(beneath)
	if err != nil {
		return nil, err
	}

	overlay, differs := difference(full, base)
	if !differs {
		overlay = map[string]any{}
	}

	return indented(overlay)
}

// difference is what full sets that base does not, key by key within objects,
// and whether there is any.
func difference(full, base any) (any, bool) {
	fullObject, fullIsObject := full.(map[string]any)
	baseObject, baseIsObject := base.(map[string]any)

	if !fullIsObject || !baseIsObject {
		return full, !reflect.DeepEqual(full, base)
	}

	differing := map[string]any{}

	for key, value := range fullObject {
		if changed, differs := difference(value, baseObject[key]); differs {
			differing[key] = changed
		}
	}

	return differing, len(differing) > 0
}

// configValue is cfg as a JSON value, its numbers kept as written.
func configValue(cfg Config) (any, error) {
	// Trade-off TRADE-13: a Config always encodes.
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("encoding configuration: %w", err)
	}

	return jsonValue(encoded)
}

// jsonValue decodes contents into plain JSON values, numbers kept as written
// so one is never rounded through a float.
func jsonValue(contents []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.UseNumber()

	var value any

	err := decoder.Decode(&value)
	if err != nil {
		return nil, fmt.Errorf("reading configuration: %w", err)
	}

	return value, nil
}

// layersRevision is the revision of the pair: the lone file's own revision
// when only one exists, so a configuration with no layers is at the revision
// it always was, and otherwise one over both, each framed with its length.
func layersRevision(home, repo layer) Revision {
	switch {
	case !home.exists && !repo.exists:
		return Revision{}
	case !repo.exists:
		return revisionOfContents(home.contents)
	case !home.exists:
		return revisionOfContents(repo.contents)
	default:
		var framed []byte

		for _, each := range []layer{home, repo} {
			framed = binary.BigEndian.AppendUint64(framed, uint64(len(each.contents)))
			framed = append(framed, each.contents...)
		}

		return revisionOfContents(framed)
	}
}
