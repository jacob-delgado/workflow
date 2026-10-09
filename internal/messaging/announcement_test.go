// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"strings"
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// testAuthor is the author the announcement-text tests render.
const testAuthor = "jacob"

func TestAMarkdownAnnouncementNeutralizesAHostileValue(t *testing.T) {
	t.Parallel()

	// A title, author, summary and URL are all anyone's to write, and they reach
	// Teams and Discord as Markdown. Unescaped, "](http://evil)" in a title ends
	// the link and injects its own, and a ")" in the URL ends the link target
	// early — so every value is escaped and the URL's ")" is percent-encoded.
	for name, kind := range map[string]config.MessagingKind{
		"teams": config.KindTeams, "discord": config.KindDiscord,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			announcement := messaging.Announcement{
				Kind:             kind,
				Author:           "a[u*thor`",
				PullRequestURL:   "https://x/pull/1)evil",
				PullRequestTitle: "t](http://evil) *x* `code`",
				IssueKey:         "PROJ-1",
				IssueSummary:     "sum[m](ary)~|#>",
				IssueURL:         "https://x/browse)1",
			}

			// Act
			got := announcement.Text()

			// Assert
			if strings.Contains(got, "](http://evil)") {
				t.Errorf("Text() = %q, want the injected Markdown link neutralized", got)
			}

			if strings.Contains(got, "pull/1)evil") || !strings.Contains(got, "%29") {
				t.Errorf("Text() = %q, want the URL's ) percent-encoded so it cannot end the link", got)
			}

			// Emphasis, code, strikethrough, spoiler, heading and quote markers all
			// come from a value anyone can write, so each is shown literally.
			for _, want := range []string{`\*`, "\\`", `\~`, `\|`, `\#`, `\>`} {
				if !strings.Contains(got, want) {
					t.Errorf("Text() = %q, want %q escaped", got, want)
				}
			}
		})
	}
}

func TestAMarkdownLinkFallsBackToTextForANonWebURL(t *testing.T) {
	t.Parallel()

	// The forge should only ever return an http(s) URL; a value with any other
	// scheme, or one that is no URL at all, is not trusted as a link target
	// and is dropped to the escaped title.
	cases := map[string]string{
		"another scheme":          "javascript:alert(1)",
		"an address not parsed":   "https://example.com/%zz",
		"an address with no host": "https:///pull/1",
	}

	for name, address := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			announcement := messaging.Announcement{
				Kind: config.KindTeams, PullRequestURL: address, PullRequestTitle: "fix",
			}

			// Act
			got := announcement.Text()

			// Assert
			if strings.Contains(got, address) || strings.Contains(got, "](") || !strings.Contains(got, "fix") {
				t.Errorf("Text() = %q, want the title alone and no link built for %q", got, address)
			}
		})
	}
}

func TestAnnouncementText(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		announcement messaging.Announcement
		want         string
	}{
		"the pull request and the issue, both linked": {
			announcement: messaging.Announcement{
				Author:           testAuthor,
				PullRequestURL:   "https://github.com/example/repo/pull/42",
				PullRequestTitle: "fix(config): redact tokens",
				IssueKey:         "PROJ-412",
				IssueSummary:     "Fix token redaction",
				IssueURL:         "https://jira.example.com/browse/PROJ-412",
			},
			want: "jacob opened a pull request: <https://github.com/example/repo/pull/42|fix(config): redact tokens>\n" +
				"<https://jira.example.com/browse/PROJ-412|PROJ-412> Fix token redaction",
		},
		"no author and no issue": {
			announcement: messaging.Announcement{PullRequestURL: "https://x/pull/1", PullRequestTitle: "docs: y"},
			want:         "A pull request is ready for review: <https://x/pull/1|docs: y>",
		},
		"an issue with no link to it": {
			announcement: messaging.Announcement{
				PullRequestURL: "https://x/pull/1", PullRequestTitle: "t", IssueKey: "OPS-1", IssueSummary: "s",
			},
			want: "A pull request is ready for review: <https://x/pull/1|t>\nOPS-1 s",
		},
		"a merged pull request, with author": {
			announcement: messaging.Announcement{
				Moment:           messaging.MomentMerged,
				Author:           testAuthor,
				PullRequestURL:   "https://x/pull/42",
				PullRequestTitle: "fix: redact tokens",
			},
			want: "jacob merged a pull request: <https://x/pull/42|fix: redact tokens>",
		},
		"a merged pull request, no author": {
			announcement: messaging.Announcement{
				Moment: messaging.MomentMerged, PullRequestURL: "https://x/pull/42", PullRequestTitle: "t",
			},
			want: "A pull request merged: <https://x/pull/42|t>",
		},
		"CI is red on the change": {
			announcement: messaging.Announcement{
				Moment: messaging.MomentCIRed, PullRequestURL: "https://x/pull/9", PullRequestTitle: "flaky",
			},
			want: "CI is red on the pull request: <https://x/pull/9|flaky>",
		},
	}

	for name, tt := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act & Assert
			if got := tt.announcement.Text(); got != tt.want {
				t.Errorf("Text() =\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

func TestAnnouncementEscapesWhatSlackWouldReadAsMarkup(t *testing.T) {
	t.Parallel()

	// Arrange
	// A title is anyone's to write. Unescaped, <!channel> pings everyone in it,
	// and a > ends the link early.
	announcement := messaging.Announcement{
		Author:           "a&b",
		PullRequestURL:   "https://x/pull/1?a=1&b=2",
		PullRequestTitle: "fix: <!channel> handle a > b",
	}

	// Act
	got := announcement.Text()

	// Assert
	if strings.Contains(got, "<!channel>") || !strings.Contains(got, "&lt;!channel&gt;") ||
		!strings.Contains(got, "a &gt; b") || !strings.Contains(got, "a&amp;b") || !strings.Contains(got, "a=1&amp;b=2") {
		t.Errorf("Text() = %q, want &, < and > escaped", got)
	}
}

func TestEveryMomentEscapesWhatSlackWouldReadAsMarkup(t *testing.T) {
	t.Parallel()

	// Every moment builds its link and lead the same way, so the escaping that
	// stops a hostile title injecting Slack markup must hold for all of them.
	for name, moment := range map[string]messaging.Moment{
		"ready": messaging.MomentReady, "merged": messaging.MomentMerged, "ci red": messaging.MomentCIRed,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Arrange
			// The author, title and URL are all hostile, so every moment that
			// includes them must escape them — an unescaped one lets a title or a
			// name inject Slack markup.
			announcement := messaging.Announcement{
				Moment: moment, Author: "a&b<!channel>",
				PullRequestURL: "https://x/pull/1?a=1&b=2", PullRequestTitle: "fix: <!here> a > b",
			}

			// Act
			got := announcement.Text()

			// Assert
			if strings.Contains(got, "<!channel>") || strings.Contains(got, "<!here>") ||
				strings.Contains(got, "a > b") || !strings.Contains(got, "&lt;!here&gt;") {
				t.Errorf("Text() = %q, want &, < and > escaped for the %s moment", got, name)
			}
		})
	}
}

func TestAConfiguredTemplateShapesOnlyAReadyAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	// The template is the team's "ready for review" wording. A merge is not ready
	// for review, so it uses the built-in merge text rather than the template.
	announcement := messaging.Announcement{
		Moment:           messaging.MomentMerged,
		Template:         "🚀 {author} needs a review of <{url}|{title}>",
		Author:           testAuthor,
		PullRequestURL:   "https://x/pull/7",
		PullRequestTitle: "fix: redact",
	}

	// Act
	got := announcement.Text()

	// Assert
	if want := "jacob merged a pull request: <https://x/pull/7|fix: redact>"; got != want {
		t.Errorf("Text() = %q, want the built-in merge text, not the template", got)
	}
}

func TestAConfiguredTemplateShapesTheAnnouncement(t *testing.T) {
	t.Parallel()

	// Arrange
	announcement := messaging.Announcement{
		Template:         "🚀 {author} needs a review of <{url}|{title}> for {key}",
		Author:           testAuthor,
		PullRequestURL:   "https://x/pull/7",
		PullRequestTitle: "fix: redact tokens",
		IssueKey:         "PROJ-412",
	}

	// Act
	got := announcement.Text()

	// Assert
	want := "🚀 jacob needs a review of <https://x/pull/7|fix: redact tokens> for PROJ-412"
	if got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestAConfiguredTemplateStillEscapesAHostileValue(t *testing.T) {
	t.Parallel()

	// Arrange
	// The template is the team's own, but a title is anyone's: a substituted
	// value must stay escaped so it cannot break out of the template and ping the
	// whole channel.
	announcement := messaging.Announcement{
		Template:         "review please: {title}",
		PullRequestTitle: "fix: <!channel> ship it",
	}

	// Act
	got := announcement.Text()

	// Assert
	if strings.Contains(got, "<!channel>") || !strings.Contains(got, "&lt;!channel&gt;") {
		t.Errorf("Text() = %q, want the title escaped so it cannot ping the channel", got)
	}
}
