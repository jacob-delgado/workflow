// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"testing"

	"github.com/jacob-delgado/workflow/internal/config"
	"github.com/jacob-delgado/workflow/internal/messaging"
)

// summaryMarkdown is a Summary as it is copied: headings five deep, a list,
// and a note, with a title that would ping a Slack channel unescaped.
const summaryMarkdown = "# 2026-10-01 to 2026-10-02\n\n## 2026\n\n### 2026-10 October\n\n" +
	"#### 2026-10-01 Thursday\n\n##### 09:00\n\n- committed abc1234 Fix <!channel> & more\n\n" +
	"Jira had more than this shows.\n"

func TestASummaryRendersForEachService(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		kind config.MessagingKind
		want string
	}{
		"Slack has no headings, so each is a bold line, every value escaped": {
			kind: config.KindSlack,
			want: "*2026-10-01 to 2026-10-02*\n\n*2026*\n\n*2026-10 October*\n\n*2026-10-01 Thursday*\n\n*09:00*\n\n" +
				"• committed abc1234 Fix &lt;!channel&gt; &amp; more\n\nJira had more than this shows.\n",
		},
		"no kind renders for Slack": {
			kind: "",
			want: "*2026-10-01 to 2026-10-02*\n\n*2026*\n\n*2026-10 October*\n\n*2026-10-01 Thursday*\n\n*09:00*\n\n" +
				"• committed abc1234 Fix &lt;!channel&gt; &amp; more\n\nJira had more than this shows.\n",
		},
		"Teams shows no heading in a webhook's text, so each is bold": {
			kind: config.KindTeams,
			want: "**2026-10-01 to 2026-10-02**\n\n**2026**\n\n**2026-10 October**\n\n**2026-10-01 Thursday**\n\n" +
				"**09:00**\n\n- committed abc1234 Fix <!channel> & more\n\nJira had more than this shows.\n",
		},
		"Discord keeps three levels of heading and bolds the rest": {
			kind: config.KindDiscord,
			want: "# 2026-10-01 to 2026-10-02\n\n## 2026\n\n### 2026-10 October\n\n**2026-10-01 Thursday**\n\n" +
				"**09:00**\n\n- committed abc1234 Fix <!channel> & more\n\nJira had more than this shows.\n",
		},
		"a plain webhook reads no markup, so a heading is its words": {
			kind: config.KindWebhook,
			want: "2026-10-01 to 2026-10-02\n\n2026\n\n2026-10 October\n\n2026-10-01 Thursday\n\n09:00\n\n" +
				"- committed abc1234 Fix <!channel> & more\n\nJira had more than this shows.\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := messaging.RenderMarkdown(tt.kind, summaryMarkdown)

			// Assert
			if got != tt.want {
				t.Errorf("RenderMarkdown(%q) =\n%s\nwant\n%s", tt.kind, got, tt.want)
			}
		})
	}
}

func TestOnlyOneToSixHashesAndASpaceOpenAHeading(t *testing.T) {
	t.Parallel()

	// Arrange
	text := "#hashtag\n####### seven deep\n indented\n- #42 Redact"

	// Act
	got := messaging.RenderMarkdown(config.KindSlack, text)

	// Assert
	if want := "#hashtag\n####### seven deep\n indented\n• #42 Redact"; got != want {
		t.Errorf("RenderMarkdown = %q, want %q", got, want)
	}
}

func TestASummarysEscapedTitleReadsAsWrittenWhereMarkdownIsNotRead(t *testing.T) {
	t.Parallel()

	const escaped = `- committed abc1234 \[Click\]\(https://evil\) a\_b \\ c`

	tests := map[string]struct {
		kind config.MessagingKind
		want string
	}{
		"Slack reads mrkdwn, not Markdown's escapes": {
			kind: config.KindSlack, want: `• committed abc1234 [Click](https://evil) a_b \ c`,
		},
		"a plain webhook reads no markup": {
			kind: config.KindWebhook, want: `- committed abc1234 [Click](https://evil) a_b \ c`,
		},
		"Teams reads Markdown, so the escapes stay":   {kind: config.KindTeams, want: escaped},
		"Discord reads Markdown, so the escapes stay": {kind: config.KindDiscord, want: escaped},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := messaging.RenderMarkdown(tt.kind, escaped)

			// Assert
			if got != tt.want {
				t.Errorf("RenderMarkdown(%q) = %q, want %q", tt.kind, got, tt.want)
			}
		})
	}
}
