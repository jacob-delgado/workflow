// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// GetConfig reads the configuration file, taking up an edit made to it since
// the server last read or wrote it, and returns the configuration in effect with
// secrets masked, and what it stands for as its ETag.
func (s *server) GetConfig(_ context.Context, _ api.GetConfigRequestObject) (api.GetConfigResponseObject, error) {
	if s.setupNeeded() {
		return api.GetConfig404ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeNotFound, noFileHere)), nil
	}

	cfg, read, err := s.reread()
	if errors.Is(err, config.ErrInvalid) {
		return api.GetConfig422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"the configuration file on disk is not valid, so the configuration in effect stands; "+
				"workflow doctor says what is wrong with it")), nil
	}

	if err != nil {
		return problemAnswer[api.GetConfigdefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
	}

	// Trade-off TRADE-13: configDTO does not fail; see there.
	out, err := configDTO(cfg)
	if err != nil {
		return problemAnswer[api.GetConfigdefaultApplicationProblemPlusJSONResponse](s.fault(err)), nil
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

	cfg, current, err := config.LoadLayersAt(s.files)
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
		s.adoptForge(cfg.Forge)
		s.adoptMessaging(cfg.Messaging)
	}

	return s.cfg, basis{seen: s.seen, gone: s.gone}, nil
}

// placeSlackCredentials is the wiring's keeping of typed Slack user-token
// secrets, with Slack's refusal of them told apart from a refused
// announcement; nil when none is wired, so the file keeps them.
func (s *server) placeSlackCredentials() func(config.Config) (config.Config, error) {
	if s.deps.PlaceSlackCredentials == nil {
		return nil
	}

	return func(incoming config.Config) (config.Config, error) {
		placed, err := s.deps.PlaceSlackCredentials(incoming)
		if errors.Is(err, messaging.ErrRejected) {
			return config.Config{}, fmt.Errorf("%w: %w", errSlackRefused, err)
		}

		return placed, err
	}
}

// adoptMessaging hands messaging settings newly in effect to every post after
// them. It runs under mu, with the configuration it belongs to.
func (s *server) adoptMessaging(settings config.Messaging) {
	if s.deps.UseMessagingSettings != nil {
		s.deps.UseMessagingSettings(settings)
	}
}

// adoptForge hands forge settings newly in effect — saved, or edited on disk
// and read — to every forge call after them, and names the forge they point
// at. It runs under mu, with the configuration it belongs to.
func (s *server) adoptForge(settings config.Forge) {
	if s.deps.UseForgeSettings != nil {
		s.forgeKind = s.deps.UseForgeSettings(settings)
	}
}

// errSlackRefused reports Slack refusing the user token's secrets typed into
// Settings, as distinct from refusing an announcement.
var errSlackRefused = errors.New("slack refused the typed user token")

// UpdateConfig writes the configuration file, but only over the revision
// If-Match names, so a change made since that read, on disk or by another
// tab's save, is refused with 409. Saves from this server are serialized, but
// the check and the write to disk are not one step (config.SaveOver), so an
// edit landing on disk between the two is written over. A stored secret is
// kept when its field comes back masked or empty. It returns the result with
// secrets masked, and the revision it wrote as its ETag.
func (s *server) UpdateConfig(
	_ context.Context, request api.UpdateConfigRequestObject,
) (api.UpdateConfigResponseObject, error) {
	if s.setupNeeded() {
		return api.UpdateConfig404ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeNotFound, noFileHere)), nil
	}

	if request.Params.IfMatch == nil {
		return api.UpdateConfig428ApplicationProblemPlusJSONResponse(problem(api.ProblemCodePreconditionRequired,
			"the save did not say which revision of the configuration it was made over; "+
				"reload the page, then save again")), nil
	}

	over, err := basisIn(*request.Params.IfMatch)
	if err != nil {
		//nolint:nilerr // an If-Match naming no revision is answered with a 400 response, not a returned error
		return problemAnswer[api.UpdateConfigdefaultApplicationProblemPlusJSONResponse](problem(api.ProblemCodeBadRequest,
			"If-Match names no revision of the configuration; send the ETag a read of it returned")), nil
	}

	return s.writeOver(*request.Body, over), nil
}

// writeOver writes a configuration posted over the API over the read over, and
// answers with what it wrote, or with why it wrote nothing.
func (s *server) writeOver(posted api.Config, over basis) api.UpdateConfigResponseObject {
	incoming, err := fromDTO(posted)
	if err != nil {
		return api.UpdateConfig422ApplicationProblemPlusJSONResponse(
			problem(api.ProblemCodeUnprocessable, invalidReason(err)))
	}

	err = s.keymapRefusal(incoming.UI.Keys)
	if err != nil {
		return api.UpdateConfig422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"the terminal interface would not start on this keymap: "+err.Error()))
	}

	saved, written, err := s.save(incoming, removedIn(posted), over)
	if errors.Is(err, config.ErrChangedOnDisk) {
		return api.UpdateConfig409ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeConflict,
			"the configuration changed since Settings read it; reload Settings and apply your change again"))
	}

	if errors.Is(err, errSlackRefused) {
		return api.UpdateConfig422ApplicationProblemPlusJSONResponse(problem(api.ProblemCodeUnprocessable,
			"Slack refused the client ID, client secret or refresh token; check them, then save again"))
	}

	if err != nil {
		return problemAnswer[api.UpdateConfigdefaultApplicationProblemPlusJSONResponse](s.fault(err))
	}

	// Trade-off TRADE-13: configDTO does not fail; see there.
	out, err := configDTO(saved)
	if err != nil {
		return problemAnswer[api.UpdateConfigdefaultApplicationProblemPlusJSONResponse](s.fault(err))
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

// GetKeys lists the terminal interface's key actions under the ui.keys of the
// configuration in effect, and whether the page's single-key shortcuts are on.
// It takes up an edit made to the file first, as a read of the configuration
// does, so a rebinding saved by hand reaches a page that reads its keys again;
// a file that is not valid, or cannot be read, leaves the configuration in
// effect standing, and its keys with it, as it does for that read.
func (s *server) GetKeys(_ context.Context, _ api.GetKeysRequestObject) (api.GetKeysResponseObject, error) {
	cfg, _, err := s.reread()
	if err != nil {
		cfg = s.config()
	}

	actions := []api.KeyAction{}

	if s.deps.KeyActions != nil {
		for _, listed := range s.deps.KeyActions(s.forgeKindNow().Noun(), cfg.Messaging.Service(), cfg.UI.Keys) {
			actions = append(actions, api.KeyAction{
				Action: listed.Action, Help: listed.Help, Group: listed.Group, Shown: listed.Shown, Keys: listed.Keys,
				Default: listed.Default,
			})
		}
	}

	return api.GetKeys200JSONResponse{SingleKeyShortcuts: cfg.UI.WebShortcuts, Actions: actions}, nil
}

// save writes incoming over the read over, through config.SaveEdit, and
// adopts it as the configuration in effect, at the revision it wrote. The
// secrets it keeps are those of the configuration in effect, which a masked
// field stands for only when that is the configuration the read showed, so a
// save is made only over the read of it: a file edited and then put back is at
// the revision named, but not at the configuration read, and two reads that
// found no file stand for different configurations when another file came and
// went between them.
func (s *server) save(
	incoming config.Config, removed []config.Credential, over basis,
) (config.Config, config.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if over != (basis{seen: s.seen, gone: s.gone}) {
		return config.Config{}, config.Revision{}, fmt.Errorf(
			"the configuration in effect is not the one read at %s: %w", over.etag(), config.ErrChangedOnDisk)
	}

	saved, written, err := config.SaveEdit(config.Edit{
		Files: s.files, Read: s.cfg, Over: over.file(), Edited: incoming, Removed: removed,
		PlaceSlackCredentials: s.placeSlackCredentials(),
	})
	if err != nil {
		return config.Config{}, config.Revision{}, err
	}

	s.cfg, s.seen, s.gone = saved, written, false
	s.adoptForge(saved.Forge)
	s.adoptMessaging(saved.Messaging)

	return saved, written, nil
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
//
// Trade-off TRADE-13: neither step fails, since both types hold only what
// encoding/json always encodes, and the one's JSON always fits the other.
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

// removedIn are the credentials a posted configuration removes: each sent as
// null.
func removedIn(posted api.Config) []config.Credential {
	var removed []config.Credential

	for credential, value := range map[config.Credential]*string{
		config.CredentialJiraToken:    posted.Jira.Token,
		config.CredentialForgeToken:   posted.Forge.Token,
		config.CredentialWebhookURL:   posted.Messaging.WebhookURL,
		config.CredentialClientSecret: posted.Messaging.ClientSecret,
		config.CredentialRefreshToken: posted.Messaging.RefreshToken,
	} {
		if value == nil {
			removed = append(removed, credential)
		}
	}

	return removed
}

// fromDTO decodes a configuration written over the API through Parse, so it is
// held to exactly the standard a file on disk is.
func fromDTO(in api.Config) (config.Config, error) {
	// Trade-off TRADE-13: an api.Config always encodes.
	data, err := json.Marshal(in)
	if err != nil {
		return config.Config{}, fmt.Errorf("encoding the posted configuration: %w", err)
	}

	return config.Parse(bytes.NewReader(data))
}
