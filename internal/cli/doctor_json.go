// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"io"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
)

// doctorReport is the whole report as data. It carries the same facts, and the
// same masking, as the prose report — no field ever holds a token, and a URL
// that could carry a credential is shown through the same helpers the prose uses.
type doctorReport struct {
	Version       string           `json:"version"`
	Repository    repositoryFacts  `json:"repository"`
	Tooling       []toolFacts      `json:"tooling"`
	Store         storeFacts       `json:"store"`
	Configuration *configFacts     `json:"configuration,omitempty"`
	ConfigProblem string           `json:"config_problem,omitempty"`
	Credentials   credentialsFacts `json:"credentials"`
}

// toolFacts is one external program and whether it was found, and what its
// probe found or why it cannot be used, where it said more.
type toolFacts struct {
	Name     string `json:"name"`
	Found    bool   `json:"found"`
	Required bool   `json:"required"`
	Effect   string `json:"effect"`
	Detail   string `json:"detail,omitempty"`
}

// configFacts is the configuration in effect.
type configFacts struct {
	Path            string   `json:"path"`
	Files           []string `json:"files"`
	Tracker         string   `json:"tracker"`
	JiraURL         string   `json:"jira_url"`
	JiraAuthMode    string   `json:"jira_auth_mode"`
	MessagingTarget string   `json:"messaging_target"`
	MessagingMode   string   `json:"messaging_mode"`
	WorldReadable   bool     `json:"world_readable"`
	Missing         []string `json:"missing"`
	Problems        []string `json:"problems"`
}

// credentialsFacts is the outcome of the online checks, or that none were run.
type credentialsFacts struct {
	Checked bool             `json:"checked"`
	Results []credentialLine `json:"results,omitempty"`
}

// runDoctorJSON writes the report as JSON and returns the same aggregate error
// the prose report would, so a script can read the exit code as well as the data.
func runDoctorJSON(ctx context.Context, out io.Writer, run doctorRun) error {
	repository, remote := repositoryFactsFor(ctx, run.env)
	tooling, toolingErr := toolingFacts(ctx, run, remote)

	report := doctorReport{
		Version: buildinfo.Current(), Repository: repository, Tooling: tooling, Store: storeFactsFor(run),
	}

	// Like the prose report, it checks no credential when the file did not
	// load: the configuration in hand is then the defaults, not the user's.
	var configErr, credErr error
	if run.loadErr != nil {
		report.ConfigProblem, configErr = run.loadErr.Error(), run.loadErr
	} else {
		facts, problem := configurationFacts(run.cfg)
		report.Configuration, configErr = &facts, problem
		report.Credentials, credErr = credentialFacts(ctx, run, remote)
	}

	err := encodeJSON(out, report)
	if err != nil {
		return err
	}

	return errors.Join(toolingErr, configErr, credErr)
}

// configurationFacts is the configuration in effect and the same review the
// prose report prints, so the JSON reaches the prose report's verdict.
func configurationFacts(cfg config.Config) (configFacts, error) {
	review := reviewConfiguration(cfg)

	return configFacts{
		Path:            cfg.Path,
		Files:           cfg.Layers().Each(),
		Tracker:         trackerOf(cfg.Jira),
		JiraURL:         config.DisplayURL(cfg.Jira.BaseURL),
		JiraAuthMode:    cfg.Jira.AuthMode().String(),
		MessagingTarget: cfg.Messaging.Target(),
		MessagingMode:   cfg.Messaging.Mode().String(),
		WorldReadable:   review.shared,
		Missing:         review.missing,
		Problems:        review.problems,
	}, review.err()
}

// credentialFacts runs the online checks through the same functions the prose
// report uses, keeping each one's line as data: one model, so the two reports
// can never mask or word a check differently.
func credentialFacts(ctx context.Context, run doctorRun, remote string) (credentialsFacts, error) {
	if !run.online {
		return credentialsFacts{Checked: false}, nil
	}

	var results []credentialLine

	err := checkCredentials(ctx, run, remote, func(line credentialLine) { results = append(results, line) })

	return credentialsFacts{Checked: true, Results: results}, err
}
