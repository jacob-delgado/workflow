// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/jacob-delgado/workflow/internal/api"
	"github.com/jacob-delgado/workflow/internal/config"
)

// GetConfig returns the configuration in effect, with secrets masked.
func (s *server) GetConfig(_ context.Context, _ api.GetConfigRequestObject) (api.GetConfigResponseObject, error) {
	out, err := configDTO(s.config())
	if err != nil {
		body, code := fault(err)

		return api.GetConfigdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.GetConfig200JSONResponse(out), nil
}

// UpdateConfig writes the configuration file, keeping a stored secret when its
// field comes back masked or empty, and returns the result with secrets masked.
func (s *server) UpdateConfig(
	_ context.Context, request api.UpdateConfigRequestObject,
) (api.UpdateConfigResponseObject, error) {
	if request.Body == nil {
		return api.UpdateConfig422JSONResponse{Code: api.Unprocessable, Message: "a configuration body is required"}, nil
	}

	incoming, err := fromDTO(*request.Body)
	if err != nil {
		//nolint:nilerr // an invalid body is answered with a 422 response, not a returned error
		return api.UpdateConfig422JSONResponse{Code: api.Unprocessable, Message: "the configuration is not valid"}, nil
	}

	saved, err := s.save(incoming)
	if err != nil {
		body, code := fault(err)

		return api.UpdateConfigdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	out, err := configDTO(saved)
	if err != nil {
		body, code := fault(err)

		return api.UpdateConfigdefaultJSONResponse{Body: body, StatusCode: code}, nil
	}

	return api.UpdateConfig200JSONResponse(out), nil
}

// save preserves the stored secrets into incoming, writes it to the file the
// configuration was read from, and adopts it as the configuration in effect.
func (s *server) save(incoming config.Config) (config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	incoming = preserveSecrets(incoming, s.cfg)
	incoming.Path = s.cfg.Path

	err := config.Save(incoming.Path, incoming)
	if err != nil {
		return config.Config{}, err
	}

	s.cfg = incoming

	return incoming, nil
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
	incoming.Jira.Token = keepSecret(incoming.Jira.Token, stored.Jira.Token)
	incoming.Slack.Token = keepSecret(incoming.Slack.Token, stored.Slack.Token)
	incoming.Slack.WebhookURL = keepSecret(incoming.Slack.WebhookURL, stored.Slack.WebhookURL)
	incoming.Forge.Token = keepSecret(incoming.Forge.Token, stored.Forge.Token)
	incoming.Jira.Headers = keepHeaders(incoming.Jira.Headers, stored.Jira.Headers)

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

// keepHeaders applies keepSecret's rule to each Jira header value, which is
// masked on read the same way a token is.
func keepHeaders(incoming, stored map[string]string) map[string]string {
	for key, value := range incoming {
		if value == "" || value == config.Redact(stored[key]) {
			incoming[key] = stored[key]
		}
	}

	return incoming
}
