// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/sanitize"
)

// typedValue is what text typed into a setting stands for: a credential's
// text, and false when it is left empty, which keeps the stored one; a
// count's count; a list's entries; and for any other kind typed into whole,
// the text itself. Only a setting typed into whole asks: a toggle or a choice
// is never typed into, and a collection's entry is kept by its own steps.
func typedValue(field setting, text string) (any, bool, error) {
	if field.kind == settingSecret {
		return text, text != "", nil
	}

	if field.kind == settingCount {
		count, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil || count < 0 {
			return nil, false, errNotACount
		}

		// A count reads back from JSON as a float64, so it is kept as one.
		return float64(count), true, nil
	}

	if field.kind == settingList {
		return splitList(text), true, nil
	}

	return text, true, nil
}

// editedText is a setting's value as its field starts: empty for a
// credential, which is never shown, and the value as shown otherwise.
func (f settingsForm) editedText(field setting) string {
	if field.kind == settingSecret {
		return ""
	}

	return f.shown(field)
}

// edited is the configuration as read with the edits laid over it, held to
// the standard a file on disk is.
func (f settingsForm) edited() (config.Config, error) {
	values := maps.Clone(f.values)

	for path, value := range f.edits {
		section, name, _ := strings.Cut(path, ".")

		// Every section is in the configuration's JSON form, which encodes
		// each one whole; the section is copied, so the read stays as read.
		inner, _ := values[section].(map[string]any)
		inner = maps.Clone(inner)
		inner[name] = value
		values[section] = inner
	}

	// Trade-off TRADE-13: the configuration's JSON form always encodes.
	encoded, err := json.Marshal(values)
	if err != nil {
		return config.Config{}, fmt.Errorf("writing the configuration: %w", err)
	}

	return parsed(encoded)
}

// parsed is a configuration's JSON form held to the standard a file on disk
// is, or why it falls short.
func parsed(read []byte) (config.Config, error) {
	cfg, err := config.Parse(bytes.NewReader(read))
	if err != nil {
		return config.Config{}, invalidSettingsError{parseErr: err}
	}

	return cfg, nil
}

// invalidSettingsError is why a configuration falls short of the standard a
// file on disk is: config.Parse's refusal, said on one line in Settings'
// words, which still names each setting it refuses, for their rows to be
// marked.
type invalidSettingsError struct {
	parseErr error
}

var _ error = invalidSettingsError{}

// Error is the refusal on one line, after Settings' own words for it in
// place of the file's name, which a configuration being edited is not yet.
func (e invalidSettingsError) Error() string {
	reason := strings.TrimPrefix(e.parseErr.Error(), config.ErrInvalid.Error()+": ")

	return errSettingsInvalid.Error() + ": " + strings.ReplaceAll(reason, "\n", "; ")
}

// Unwrap is Settings' kind of refusal and config.Parse's own, so
// errors.Is finds either and config.FieldErrors each setting refused.
func (e invalidSettingsError) Unwrap() []error {
	return []error{errSettingsInvalid, e.parseErr}
}

// seed is a configuration's JSON form, read back as values, or why it could
// not be had.
func seed(cfg config.Config, err error) (map[string]any, error) {
	if err != nil {
		return nil, err
	}

	// Trade-off TRADE-13: a Config always encodes, and its JSON always reads
	// back.
	var values map[string]any

	read, err := json.Marshal(cfg)
	if err == nil {
		err = json.Unmarshal(read, &values)
	}

	if err != nil {
		return nil, fmt.Errorf("reading the configuration: %w", err)
	}

	return values, nil
}

// valueAt is the value at a dotted path in a configuration's JSON form, or
// nil where there is none.
func valueAt(values map[string]any, path string) any {
	section, name, _ := strings.Cut(path, ".")
	inner, _ := values[section].(map[string]any)

	return inner[name]
}

// shown is a setting's value as the screen may show it: a credential masked,
// whatever was typed for it.
func (f settingsForm) shown(field setting) string {
	value := f.value(field.path)
	text, _ := value.(string)

	// The row that adds an entry holds no value of its own, so it shows the
	// empty text it reads as.
	switch field.kind {
	case settingText, settingToggle, settingAdd:
		return sanitize.Line(text)
	case settingSecret:
		if _, typed := f.edits[field.path]; typed {
			return "new value, hidden"
		}

		return config.Redact(text)
	case settingURL:
		return sanitize.Line(config.RedactURL(text))
	case settingCount:
		count, _ := value.(float64)

		return strconv.Itoa(int(count))
	case settingList:
		return sanitize.Line(strings.Join(listOf(value), ", "))
	case settingChoice:
		return field.words(text)
	case settingEntry:
		return f.shownEntry(field)
	}

	return sanitize.Line(text)
}

// listOf is a list setting's entries, as read or as typed.
func listOf(value any) []string {
	if typed, ok := value.([]string); ok {
		return typed
	}

	read, _ := value.([]any)
	entries := make([]string, 0, len(read))

	for _, entry := range read {
		text, _ := entry.(string)
		entries = append(entries, text)
	}

	return entries
}
