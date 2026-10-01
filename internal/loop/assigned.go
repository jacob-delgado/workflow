// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"fmt"
	"slices"

	"github.com/jacob-delgado/workflow/internal/convention"
	"github.com/jacob-delgado/workflow/internal/jira"
)

// keysPerAsk is how many keys one search names. The query rides in a URL,
// which a server caps near 8 KB, and a key takes about 13 bytes of it.
const keysPerAsk = 100

// AssignedKeys is the subset of keys naming issues assigned to whoever the
// credential belongs to, asked of the tracker in batches a URL can hold and in
// pages of its size. No keys asks nothing. A forge issue's number is never
// named in the query, since Jira would read it as the id of an unrelated
// issue: the forge's own issues ignore the query and answer every assigned
// issue, alone or ahead of Jira's, so the intersection finds them all the
// same, and keys that are all forge numbers are asked once with NoKeys.
func AssignedKeys(
	search func(jql string, startAt int) (jira.SearchResult, error), keys []jira.Key,
) (map[jira.Key]bool, error) {
	asked := slices.Compact(slices.Sorted(slices.Values(keys)))
	mine := make(map[jira.Key]bool)

	for _, jql := range assignedQueries(asked) {
		found, err := assignedAmong(search, jql, asked)
		if err != nil {
			return nil, err
		}

		for _, key := range found {
			mine[key] = true
		}
	}

	return mine, nil
}

// assignedQueries is the query for each batch of asked's keys a tracker can
// read, or NoKeys alone when every key is a forge issue's.
func assignedQueries(asked []jira.Key) []string {
	named := slices.DeleteFunc(slices.Clone(asked), isForgeNumber)
	if len(named) == 0 && len(asked) > 0 {
		return []string{jira.NoKeys}
	}

	queries := make([]string, 0, len(named)/keysPerAsk+1)
	for batch := range slices.Chunk(named, keysPerAsk) {
		queries = append(queries, jira.KeysAssignedToMe(batch))
	}

	return queries
}

// isForgeNumber reports a key shaped as a forge issue's number.
func isForgeNumber(key jira.Key) bool {
	ref, known := convention.RefOf(string(key))

	return known && ref.Tracker == convention.TrackerForge
}

// assignedAmong is which of asked, sorted, the tracker answers jql with, page
// by page until its total is reached or a page comes back empty.
func assignedAmong(
	search func(jql string, startAt int) (jira.SearchResult, error), jql string, asked []jira.Key,
) ([]jira.Key, error) {
	var found []jira.Key

	for startAt := 0; ; {
		page, err := search(jql, startAt)
		if err != nil {
			return nil, fmt.Errorf("asking which issues are assigned to you: %w", err)
		}

		for _, issue := range page.Issues {
			if _, known := slices.BinarySearch(asked, issue.Key); known {
				found = append(found, issue.Key)
			}
		}

		if len(page.Issues) == 0 || startAt+len(page.Issues) >= page.Total {
			return found, nil
		}

		startAt += len(page.Issues)
	}
}
