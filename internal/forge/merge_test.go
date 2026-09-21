// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package forge_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/jacob-delgado/workflow/internal/forge"
)

func TestMergePullRequestOnGitHubSendsTheMethod(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusOK, `{"merged":true}`)

	// Act
	err := client.Merge(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, forge.MergeSquash)
	// Assert
	if err != nil {
		t.Fatalf("Merge returned %v", err)
	}

	asked := lastRequest(t, seen)
	if asked.method != http.MethodPut || asked.path != githubPullsPath+"/42/merge" {
		t.Errorf("asked %s %s, want PUT the merge path", asked.method, asked.path)
	}

	if asked.body["merge_method"] != "squash" {
		t.Errorf("merge_method = %v, want squash", asked.body["merge_method"])
	}
}

func TestMergeMergeRequestOnGitLabSquashesOnlyForSquash(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		method forge.MergeMethod
		squash bool
	}{
		"a squash squashes":       {method: forge.MergeSquash, squash: true},
		"a merge commit does not": {method: forge.MergeCommit, squash: false},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, seen := forgeAnswering(t, http.StatusOK, `{"state":"merged"}`)

			// Act
			err := client.Merge(t.Context(), gitlabRepo(), forge.PullRequest{Number: 8}, tt.method)
			// Assert
			if err != nil {
				t.Fatalf("Merge returned %v", err)
			}

			asked := lastRequest(t, seen)
			if asked.method != http.MethodPut || asked.path != gitlabMergesPath+"/8/merge" {
				t.Errorf("asked %s %s, want PUT the merge path", asked.method, asked.path)
			}

			if asked.body["squash"] != tt.squash {
				t.Errorf("squash = %v, want %v", asked.body["squash"], tt.squash)
			}
		})
	}
}

func TestMergeMethodsOnGitHubReadsTheAllowFlags(t *testing.T) {
	t.Parallel()

	// Arrange
	client, seen := forgeAnswering(t, http.StatusOK,
		`{"allow_merge_commit":true,"allow_squash_merge":false,"allow_rebase_merge":true}`)

	// Act
	methods, err := client.MergeMethods(t.Context(), githubRepo())

	// Assert
	if err != nil || !slices.Equal(methods, []forge.MergeMethod{forge.MergeCommit, forge.MergeRebase}) {
		t.Errorf("MergeMethods = %v, %v, want a merge commit and a rebase", methods, err)
	}

	if asked := lastRequest(t, seen); asked.path != "/repos/example/repo" {
		t.Errorf("read the settings from %q, want the repository object", asked.path)
	}
}

func TestMergeMethodsOnGitLabReadsTheProjectSettings(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		body string
		want []forge.MergeMethod
	}{
		"a rebase project with optional squash": {
			body: `{"merge_method":"rebase_merge","squash_option":"default_off"}`,
			want: []forge.MergeMethod{forge.MergeRebase, forge.MergeSquash},
		},
		"squash always": {
			body: `{"merge_method":"merge","squash_option":"always"}`,
			want: []forge.MergeMethod{forge.MergeSquash},
		},
		"squash never": {
			body: `{"merge_method":"merge","squash_option":"never"}`,
			want: []forge.MergeMethod{forge.MergeCommit},
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, http.StatusOK, tt.body)

			// Act
			methods, err := client.MergeMethods(t.Context(), gitlabRepo())

			// Assert
			if err != nil || !slices.Equal(methods, tt.want) {
				t.Errorf("MergeMethods = %v, %v, want %v", methods, err, tt.want)
			}
		})
	}
}

func TestMergeAndItsMethodsNeedAForgeThatIsKnown(t *testing.T) {
	t.Parallel()

	unknown := forge.Repo{Kind: forge.KindUnknown, Host: "example.com", Path: "x/y"}

	cases := map[string]func(forge.Client) error{
		"merge": func(client forge.Client) error {
			return client.Merge(t.Context(), unknown, forge.PullRequest{Number: 1}, forge.MergeCommit)
		},
		"merge methods": func(client forge.Client) error {
			_, err := client.MergeMethods(t.Context(), unknown)

			return err
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			client, _ := forgeAnswering(t, http.StatusOK, `{}`)

			// Act
			err := act(client)

			// Assert
			if !errors.Is(err, forge.ErrUnknownForge) {
				t.Errorf("on an unknown forge = %v, want ErrUnknownForge", err)
			}
		})
	}
}

func TestMergeMethodsSurfacesAReadFailure(t *testing.T) {
	t.Parallel()

	cases := map[string]forge.Repo{"github": githubRepo(), "gitlab": gitlabRepo()}

	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The token cannot read the repository's merge settings.
			client, _ := forgeAnswering(t, http.StatusForbidden, `{"message":"nope"}`)

			// Act
			_, err := client.MergeMethods(t.Context(), repo)

			// Assert
			if !errors.Is(err, forge.ErrRefused) {
				t.Errorf("MergeMethods = %v, want ErrRefused", err)
			}
		})
	}
}

func TestARefusedMergeSaysWhy(t *testing.T) {
	t.Parallel()

	// Arrange
	// The token can read the pull but not merge it, as an under-scoped one does.
	client, _ := forgeAnswering(t, http.StatusForbidden, `{"message":"not allowed"}`)

	// Act
	err := client.Merge(t.Context(), githubRepo(), forge.PullRequest{Number: 42}, forge.MergeCommit)

	// Assert
	if !errors.Is(err, forge.ErrRefused) {
		t.Errorf("Merge returned %v, want ErrRefused", err)
	}
}
