// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/proc"
)

// doctorReport is the whole report as data. It carries the same facts, and the
// same masking, as the prose report — no field ever holds a token, and a URL
// that could carry a credential is shown through the same helpers the prose uses.
type doctorReport struct {
	Version       string           `json:"version"`
	Repository    repositoryFacts  `json:"repository"`
	Tooling       []toolFacts      `json:"tooling"`
	Configuration *configFacts     `json:"configuration,omitempty"`
	ConfigProblem string           `json:"config_problem,omitempty"`
	Credentials   credentialsFacts `json:"credentials"`
}

// repositoryFacts is the git repository the working directory is in.
type repositoryFacts struct {
	InsideWorkTree bool   `json:"inside_work_tree"`
	Root           string `json:"root,omitempty"`
	Branch         string `json:"branch,omitempty"`
	Detached       bool   `json:"detached"`
	Remote         string `json:"remote,omitempty"`
	Forge          string `json:"forge,omitempty"`
}

// toolFacts is one external program and whether it was found.
type toolFacts struct {
	Name     string `json:"name"`
	Found    bool   `json:"found"`
	Required bool   `json:"required"`
	Effect   string `json:"effect"`
}

// configFacts is the configuration in effect.
type configFacts struct {
	Path          string   `json:"path"`
	JiraURL       string   `json:"jira_url"`
	JiraAuthMode  string   `json:"jira_auth_mode"`
	SlackTarget   string   `json:"slack_target"`
	SlackMode     string   `json:"slack_mode"`
	WorldReadable bool     `json:"world_readable"`
	Missing       []string `json:"missing"`
}

// credentialsFacts is the outcome of the online checks, or that none were run.
type credentialsFacts struct {
	Checked bool             `json:"checked"`
	Results []credentialLine `json:"results,omitempty"`
}

// credentialLine is one service's check: its status, and the same masked detail
// the prose report shows.
type credentialLine struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Detail  string `json:"detail"`
}

// runDoctorJSON writes the report as JSON and returns the same aggregate error
// the prose report would, so a script can read the exit code as well as the data.
func runDoctorJSON(ctx context.Context, out io.Writer, cfg config.Config, loadErr error, online bool) error {
	repository, remote := repositoryFactsFor(ctx)
	tooling, toolingErr := toolingFacts()

	report := doctorReport{Version: buildinfo.Current(), Repository: repository, Tooling: tooling}

	configErr := loadErr
	if loadErr != nil {
		report.ConfigProblem = loadErr.Error()
	} else {
		facts, problem := configurationFacts(cfg)
		report.Configuration, configErr = &facts, problem
	}

	credentials, credErr := credentialFacts(ctx, cfg, remote, online)
	report.Credentials = credentials

	err := encodeReport(out, report)
	if err != nil {
		return err
	}

	if loadErr != nil {
		return errors.Join(toolingErr, configErr)
	}

	return errors.Join(toolingErr, configErr, credErr)
}

// encodeReport writes the report as indented JSON.
func encodeReport(out io.Writer, report doctorReport) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")

	err := encoder.Encode(report)
	if err != nil {
		return fmt.Errorf("encoding the report: %w", err)
	}

	return nil
}

// repositoryFactsFor gathers the git facts, returning the raw remote alongside
// so the credential check can parse it — the facts carry only the masked form.
func repositoryFactsFor(ctx context.Context) (repositoryFacts, string) {
	dir, err := os.Getwd()
	if err != nil {
		return repositoryFacts{InsideWorkTree: false}, ""
	}

	repo, err := gitrepo.Describe(ctx, proc.Run, dir)
	if err != nil {
		return repositoryFacts{InsideWorkTree: false}, ""
	}

	return repositoryFacts{
		InsideWorkTree: true,
		Root:           repo.Root,
		Branch:         repo.Branch,
		Detached:       repo.Detached,
		Remote:         config.DisplayURL(repo.Remote),
		Forge:          forgeLabel(repo.Remote),
	}, repo.Remote
}

// toolingFacts lists the external programs and names any required one absent.
func toolingFacts() ([]toolFacts, error) {
	facts := make([]toolFacts, 0, len(externalTools()))

	var missing []string

	for _, program := range externalTools() {
		installed := proc.Available(program.name)
		facts = append(facts, toolFacts{
			Name: program.name, Found: installed, Required: program.required, Effect: program.effect,
		})

		if !installed && program.required {
			missing = append(missing, program.name)
		}
	}

	if len(missing) > 0 {
		return facts, fmt.Errorf("%w: %s", errMissingTooling, strings.Join(missing, ", "))
	}

	return facts, nil
}

// configurationFacts gathers the configuration in effect and names any problem
// with it — a world-readable file, or a required field still empty.
func configurationFacts(cfg config.Config) (configFacts, error) {
	mode, shared := config.SharedMode(cfg.Path)
	missing := cfg.Missing()

	facts := configFacts{
		Path:          cfg.Path,
		JiraURL:       config.DisplayURL(cfg.Jira.BaseURL),
		JiraAuthMode:  cfg.Jira.AuthMode().String(),
		SlackTarget:   cfg.Slack.Target(),
		SlackMode:     cfg.Slack.Mode().String(),
		WorldReadable: shared,
		Missing:       missing,
	}

	var problems []error

	if shared {
		problems = append(problems, fmt.Errorf("%w: mode %#o", errShared, mode))
	}

	if len(missing) > 0 {
		problems = append(problems, fmt.Errorf("%w: %d field(s) missing", errIncomplete, len(missing)))
	}

	return facts, errors.Join(problems...)
}

// credentialFacts runs the online checks through the same functions the prose
// report uses, capturing their already-masked output as data. Reusing them is
// what keeps the two reports from ever masking differently.
func credentialFacts(ctx context.Context, cfg config.Config, remote string, online bool) (credentialsFacts, error) {
	if !online {
		return credentialsFacts{Checked: false}, nil
	}

	checks := []struct {
		service string
		run     func(io.Writer) error
	}{
		{service: "jira", run: func(out io.Writer) error { return checkJira(ctx, out, cfg.Jira) }},
		{service: "slack", run: func(out io.Writer) error { return checkSlack(ctx, out, cfg.Slack) }},
		{service: "forge", run: func(out io.Writer) error { return checkForge(ctx, out, cfg.Forge, remote) }},
	}

	results := make([]credentialLine, 0, len(checks))

	var failures []error

	for _, check := range checks {
		line, err := captureCheck(check.service, check.run)
		results = append(results, line)
		failures = append(failures, err)
	}

	return credentialsFacts{Checked: true, Results: results}, errors.Join(failures...)
}

// captureCheck runs one prose check into a buffer and repackages its outcome as
// data. The buffer holds an already-masked line, so nothing a token touches
// escapes here that the prose report would not print itself.
func captureCheck(service string, check func(io.Writer) error) (credentialLine, error) {
	var buffer strings.Builder

	err := check(&buffer)
	detail := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(buffer.String()), service))

	return credentialLine{Service: service, Status: credentialStatus(err), Detail: detail}, err
}

// credentialStatus names an outcome for the reader to act on: a working
// credential, one the service refused, or a service that never answered.
func credentialStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errUnreachable):
		return "unreachable"
	default:
		return "rejected"
	}
}
