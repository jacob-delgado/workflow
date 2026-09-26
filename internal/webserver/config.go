// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
)

// GetConfig reads the configuration file, taking up an edit made to it since
// the server last read or wrote it, and returns the configuration in effect with
// secrets masked, and what it stands for as its ETag.
func (s *server) GetConfig(_ context.Context, _ api.GetConfigRequestObject) (api.GetConfigResponseObject, error) {
	cfg, read, err := s.reread()
	if errors.Is(err, config.ErrInvalid) {
		return api.GetConfig422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable,
			"the configuration file on disk is not valid, so the configuration in effect stands; "+
				"workflow doctor says what is wrong with it")), nil
	}

	if err != nil {
		body, code := fault(err)

		return api.GetConfigdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	out, err := configDTO(cfg)
	if err != nil {
		body, code := fault(err)

		return api.GetConfigdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetConfig200JSONResponse{Body: out, Headers: api.GetConfig200ResponseHeaders{ETag: read.etag()}}, nil
}

// reread takes up the configuration file when it is at a revision other than
// the one the server last read or wrote, and returns the configuration in
// effect with what it stands for. The file is read once, so the revision taken
// up is always that of the configuration taken up with it. A file that is not
// valid is refused, and the configuration in effect stands. So it does for a
// file that is gone, which is remembered as gone, so a save made over that read
// puts it back.
func (s *server) reread() (config.Config, basis, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg, current, err := config.LoadFileAt(s.path)
	if err != nil {
		return config.Config{}, basis{}, err
	}

	switch {
	case current == s.seen:
		s.gone = false
	case !current.Exists():
		s.gone = true
	default:
		s.cfg, s.seen, s.gone = cfg, current, false
	}

	return s.cfg, basis{seen: s.seen, gone: s.gone}, nil
}

// UpdateConfig writes the configuration file, but only over the revision
// If-Match names, so a change made since that read, on disk or by another
// tab's save, is refused rather than overwritten. A stored secret is kept when
// its field comes back masked or empty. It returns the result with secrets
// masked, and the revision it wrote as its ETag.
func (s *server) UpdateConfig(
	_ context.Context, request api.UpdateConfigRequestObject,
) (api.UpdateConfigResponseObject, error) {
	if request.Params.IfMatch == nil {
		return api.UpdateConfig428ApplicationProblemPlusJSONResponse(problem(api.PreconditionRequired,
			"the save did not say which revision of the configuration it was made over; "+
				"reload the page, then save again")), nil
	}

	over, err := basisIn(*request.Params.IfMatch)
	if err != nil {
		//nolint:nilerr // an If-Match naming no revision is answered with a 400 response, not a returned error
		return api.UpdateConfigdefaultApplicationProblemPlusJSONResponse{
			Body: problem(api.BadRequest,
				"If-Match names no revision of the configuration; send the ETag a read of it returned"),
			StatusCode: http.StatusBadRequest,
		}, nil
	}

	if request.Body == nil {
		return api.UpdateConfig422ApplicationProblemPlusJSONResponse(
			problem(api.Unprocessable, "a configuration body is required")), nil
	}

	return s.writeOver(*request.Body, over), nil
}

// writeOver writes a configuration posted over the API over the read over, and
// answers with what it wrote, or with why it wrote nothing.
func (s *server) writeOver(posted api.Config, over basis) api.UpdateConfigResponseObject {
	incoming, err := fromDTO(posted)
	if err != nil {
		return api.UpdateConfig422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable, invalidReason(err)))
	}

	err = s.keymapRefusal(incoming.UI.Keys)
	if err != nil {
		return api.UpdateConfig422ApplicationProblemPlusJSONResponse(problem(api.Unprocessable,
			"the terminal interface would not start on this keymap: "+err.Error()))
	}

	saved, written, err := s.save(incoming, over)
	if errors.Is(err, config.ErrChangedOnDisk) {
		return api.UpdateConfig409ApplicationProblemPlusJSONResponse(problem(api.Conflict,
			"the configuration changed since Settings read it; reload Settings and apply your change again"))
	}

	if err != nil {
		body, code := fault(err)

		return api.UpdateConfigdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}

	out, err := configDTO(saved)
	if err != nil {
		body, code := fault(err)

		return api.UpdateConfigdefaultApplicationProblemPlusJSONResponse{Body: body, StatusCode: code}
	}

	return api.UpdateConfig200JSONResponse{
		Body: out, Headers: api.UpdateConfig200ResponseHeaders{ETag: basis{seen: written}.etag()},
	}
}

// invalidReason says why a posted configuration was refused, in Parse's own
// words after the prefix naming the file, which a posted configuration is not
// yet: the setting and the value a validator refused, none of which is a
// credential, or the field the decoder could not take. Several reasons share
// the one line.
func invalidReason(err error) string {
	reason := strings.TrimPrefix(err.Error(), config.ErrInvalid.Error()+": ")

	return "the configuration is not valid: " + strings.ReplaceAll(reason, "\n", "; ")
}

// keymapRefusal is why the terminal interface would refuse keys, so a save
// never writes a file the interface will not start on; nil where it would
// start, or where no check is wired.
func (s *server) keymapRefusal(keys map[string]string) error {
	if s.deps.CheckKeys == nil {
		return nil
	}

	return s.deps.CheckKeys(keys)
}

// save preserves the stored secrets into incoming, writes it to the file the
// configuration was read from while that file is still as the read over found
// it, and adopts it as the configuration in effect, at the revision it wrote.
// The secrets it keeps are those of the configuration in effect, which a
// masked field stands for only when that is the configuration the read showed,
// so a save is made only over the read of it: a file edited and then put back
// is at the revision named, but not at the configuration read, and two reads
// that found no file stand for different configurations when another file came
// and went between them.
func (s *server) save(incoming config.Config, over basis) (config.Config, config.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if over != (basis{seen: s.seen, gone: s.gone}) {
		return config.Config{}, config.Revision{}, fmt.Errorf(
			"the configuration in effect is not the one read at %s: %w", over.etag(), config.ErrChangedOnDisk)
	}

	incoming = preserveSecrets(incoming, s.cfg)
	incoming.Path = s.path

	written, err := config.SaveOver(s.path, incoming, over.file())
	if err != nil {
		return config.Config{}, config.Revision{}, err
	}

	s.cfg, s.seen, s.gone = incoming, written, false

	return incoming, written, nil
}

// basis is what a read of the configuration stood for: the revision of the
// file the configuration in effect was last read from or written as, and
// whether the read found that file gone. Its entity tag names both, so two
// reads that found no file carry different ETags when they served different
// configurations.
type basis struct {
	seen config.Revision
	gone bool
}

// etag is the basis as an entity tag (RFC 9110): the revision's text form,
// after "none-" when the file was gone, quoted. A server that started with no
// file has read none, so its tag is plain "none".
func (b basis) etag() string {
	if b.gone {
		return `"none-` + b.seen.String() + `"`
	}

	return `"` + b.seen.String() + `"`
}

// file is the revision the file was at when the read was made: no file, when
// the read found it gone.
func (b basis) file() config.Revision {
	if b.gone {
		return config.Revision{}
	}

	return b.seen
}

// basisIn reads the basis an If-Match header names: one entity tag, as etag
// writes it. A read finds the file gone only after reading one, so etag never
// marks the no-file revision gone, and a tag that does names no read.
func basisIn(ifMatch string) (basis, error) {
	text, opened := strings.CutPrefix(ifMatch, `"`)
	text, closed := strings.CutSuffix(text, `"`)

	if !opened || !closed {
		return basis{}, config.ErrNotARevision
	}

	text, gone := strings.CutPrefix(text, "none-")

	seen, err := config.ParseRevision(text)
	if err != nil {
		return basis{}, err
	}

	if gone && !seen.Exists() {
		return basis{}, config.ErrNotARevision
	}

	return basis{seen: seen, gone: gone}, nil
}

// configDTO maps a configuration onto its wire shape, masked, through the shared
// JSON form the two representations were designed to agree on.
func configDTO(cfg config.Config) (api.Config, error) {
	data, err := json.Marshal(cfg.Redacted())
	if err != nil {
		return api.Config{}, fmt.Errorf("marshaling the configuration: %w", err)
	}

	var out api.Config

	err = json.Unmarshal(data, &out)
	if err != nil {
		return api.Config{}, fmt.Errorf("mapping the configuration to its wire shape: %w", err)
	}

	return out, nil
}

// fromDTO decodes a configuration written over the API through Parse, so it is
// held to exactly the standard a file on disk is.
func fromDTO(in api.Config) (config.Config, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return config.Config{}, fmt.Errorf("encoding the posted configuration: %w", err)
	}

	return config.Parse(bytes.NewReader(data))
}

// preserveSecrets keeps each stored secret when its incoming field is empty or
// still the masked value the read returned — a config editor sends the masked
// form back unchanged, and must not overwrite the real secret with the mask.
func preserveSecrets(incoming, stored config.Config) config.Config {
	incoming.Jira.BaseURL = keepMaskedURL(incoming.Jira.BaseURL, stored.Jira.BaseURL)
	incoming.Jira.Token = keepSecret(incoming.Jira.Token, stored.Jira.Token)
	incoming.Messaging.Token = keepSecret(incoming.Messaging.Token, stored.Messaging.Token)
	incoming.Messaging.WebhookURL = keepSecret(incoming.Messaging.WebhookURL, stored.Messaging.WebhookURL)
	incoming.Forge.Token = keepSecret(incoming.Forge.Token, stored.Forge.Token)
	incoming.Jira.Headers = keepHeaders(incoming.Jira.Headers, stored.Jira.Headers)

	return incoming
}

// keepMaskedURL keeps the stored base URL when the incoming one is only its
// masked form. jira.base_url may carry userinfo (it becomes Basic auth), which
// the read masks like any other credential; the config editor sends that masked
// URL back unchanged, and it must not overwrite the real password with the mask.
// A genuinely edited URL differs from the mask and is taken as sent.
func keepMaskedURL(incoming, stored string) string {
	if incoming == config.RedactURL(stored) {
		return stored
	}

	return incoming
}

// keepSecret returns the stored secret when the incoming one is empty or the
// mask of the stored value, and the incoming one otherwise.
func keepSecret(incoming, stored config.Secret) config.Secret {
	value := incoming.Reveal()
	if value == "" || value == config.Redact(stored.Reveal()) {
		return stored
	}

	return incoming
}

// keepHeaders applies keepSecret to each Jira header value, which is masked on
// read the same way a token is.
func keepHeaders(incoming, stored map[string]config.Secret) map[string]config.Secret {
	for key, value := range incoming {
		incoming[key] = keepSecret(value, stored[key])
	}

	return incoming
}
