// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/jacob-delgado/workflow/internal/buildinfo"
	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/gitrepo"
	"github.com/jacob-delgado/workflow/internal/messaging"
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
	Path            string   `json:"path"`
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

// credentialLine is one service's check: its status, and the same masked detail
// the prose report shows.
type credentialLine struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Detail  string `json:"detail"`
}

// runDoctorJSON writes the report as JSON and returns the same aggregate error
// the prose report would, so a script can read the exit code as well as the data.
func runDoctorJSON(ctx context.Context, out io.Writer, run doctorRun) error {
	repository, remote := repositoryFactsFor(ctx)
	tooling, toolingErr := toolingFacts()

	report := doctorReport{Version: buildinfo.Current(), Repository: repository, Tooling: tooling}

	configErr := run.loadErr
	if run.loadErr != nil {
		report.ConfigProblem = run.loadErr.Error()
	} else {
		facts, problem := configurationFacts(run.cfg)
		report.Configuration, configErr = &facts, problem
	}

	credentials, credErr := credentialFacts(ctx, run, remote)
	report.Credentials = credentials

	err := encodeJSON(out, report)
	if err != nil {
		return err
	}

	return errors.Join(toolingErr, configErr, credErr)
}

// repositoryFactsFor gathers the git facts, returning the raw remote alongside
// so the credential check can parse it — the facts carry only the masked form.
func repositoryFactsFor(ctx context.Context) (repositoryFacts, string) {
	dir, err := os.Getwd()
	if err != nil {
		return repositoryFacts{InsideWorkTree: false}, ""
	}

	repo, err := gitrepo.At(proc.Run, dir).Describe(ctx)
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

// configurationFacts is the configuration in effect and the same review the
// prose report prints, so the JSON reaches the prose report's verdict.
func configurationFacts(cfg config.Config) (configFacts, error) {
	review := reviewConfiguration(cfg)

	return configFacts{
		Path:            cfg.Path,
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
// report uses, capturing their already-masked output as data. Reusing them is
// what keeps the two reports from ever masking differently.
//
// Like the prose report, it checks nothing when the file did not load: the
// configuration in hand is then the defaults, not the user's.
func credentialFacts(ctx context.Context, run doctorRun, remote string) (credentialsFacts, error) {
	if !run.online || run.loadErr != nil {
		//nolint:nilerr // runDoctorJSON returns the load error as the configuration's problem
		return credentialsFacts{Checked: false}, nil
	}

	cfg, doers := run.cfg, onlineDoers(run.cfg, run.log)

	checks := []struct {
		service string
		run     func(io.Writer) error
	}{
		{service: "jira", run: func(out io.Writer) error { return checkJira(ctx, out, doers.jira, cfg.Jira) }},
		{service: strings.ToLower(cfg.Messaging.Service()), run: func(out io.Writer) error {
			return checkMessaging(ctx, out, doers.messaging, messaging.APIBase, cfg.Messaging)
		}},
		{service: "forge", run: func(out io.Writer) error { return checkForge(ctx, out, run, remote) }},
	}

	results := make([]credentialLine, 0, len(checks))
	outcomes := make([]error, 0, len(checks))

	for _, check := range checks {
		line, err := captureCheck(check.service, check.run)
		results = append(results, line)
		outcomes = append(outcomes, err)
	}

	return credentialsFacts{Checked: true, Results: results}, credentialVerdict(outcomes...)
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
// credential, one doctor could not ask about, none to ask with, one the service
// refused, or a service that never answered.
func credentialStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errUnchecked):
		return "unchecked"
	case errors.Is(err, errCredentialMissing):
		return "missing"
	case errors.Is(err, errUnreachable):
		return "unreachable"
	default:
		return "rejected"
	}
}
