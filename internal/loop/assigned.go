// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package loop

import (
	"fmt"
	"slices"

	"github.com/jacob-delgado/workflow/internal/jira"
)

// keysPerAsk is how many keys one search names. The query rides in a URL,
// which a server caps near 8 KB, and a key takes about 13 bytes of it.
const keysPerAsk = 100

// AssignedKeys is the subset of keys naming issues assigned to whoever the
// credential belongs to, asked of the tracker in batches a URL can hold and in
// pages of its size. No keys asks nothing. A tracker that ignores the query —
// the forge's own issues — still answers only assigned issues, so the
// intersection is right there too.
func AssignedKeys(
	search func(jql string, startAt int) (jira.SearchResult, error), keys []jira.Key,
) (map[jira.Key]bool, error) {
	asked := slices.Compact(slices.Sorted(slices.Values(keys)))
	mine := make(map[jira.Key]bool)

	for batch := range slices.Chunk(asked, keysPerAsk) {
		found, err := assignedAmong(search, batch)
		if err != nil {
			return nil, err
		}

		for _, key := range found {
			mine[key] = true
		}
	}

	return mine, nil
}

// assignedAmong is which of batch, sorted, the tracker answers as assigned,
// page by page until its total is reached or a page comes back empty.
func assignedAmong(
	search func(jql string, startAt int) (jira.SearchResult, error), batch []jira.Key,
) ([]jira.Key, error) {
	jql := jira.KeysAssignedToMe(batch)

	var found []jira.Key

	for startAt := 0; ; {
		page, err := search(jql, startAt)
		if err != nil {
			return nil, fmt.Errorf("asking which issues are assigned to you: %w", err)
		}

		for _, issue := range page.Issues {
			if _, asked := slices.BinarySearch(batch, issue.Key); asked {
				found = append(found, issue.Key)
			}
		}

		if len(page.Issues) == 0 || startAt+len(page.Issues) >= page.Total {
			return found, nil
		}

		startAt += len(page.Issues)
	}
}
