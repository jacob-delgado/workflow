// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package messaging_test

import (
	"errors"
	"strings"
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

// characters is the unit a service that counts characters is measured in.
const characters = "characters"

func TestAMessageIsMeasuredAsItsServiceMeasuresIt(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		kind config.MessagingKind
		text string
		want messaging.Length
	}{
		"Discord counts characters, not bytes, up to 2000": {
			kind: config.KindDiscord, text: strings.Repeat("é", 2001),
			want: messaging.Length{Service: "Discord", Count: 2001, Limit: 2000, Unit: characters},
		},
		"Slack counts characters up to 40000": {
			kind: config.KindSlack, text: strings.Repeat("a", 12),
			want: messaging.Length{Service: "Slack", Count: 12, Limit: 40000, Unit: characters},
		},
		"Teams counts the bytes of the whole payload up to 28000": {
			kind: config.KindTeams, text: "é",
			want: messaging.Length{Service: "Teams", Count: len(`{"text":"é"}`), Limit: 28000, Unit: "bytes"},
		},
		"a plain webhook takes any length": {
			kind: config.KindWebhook, text: strings.Repeat("a", 50000),
			want: messaging.Length{Service: "Webhook", Count: 50000, Limit: 0, Unit: characters},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			got := messaging.MessageLength(tt.kind, tt.text)

			// Assert
			if got != tt.want {
				t.Errorf("MessageLength(%q) = %+v, want %+v", tt.kind, got, tt.want)
			}
		})
	}
}

func TestATooLongMessageSaysHowLongAgainstWhat(t *testing.T) {
	t.Parallel()

	// Arrange
	length := messaging.MessageLength(config.KindDiscord, strings.Repeat("a", 2345))

	// Act
	err := length.Check()

	// Assert
	want := "too long for Discord (2345 of 2000 characters); pick a shorter period"
	if !errors.Is(err, messaging.ErrTooLong) || err.Error() != want {
		t.Errorf("Check = %v, want ErrTooLong saying %q", err, want)
	}
}

func TestAMessageWithinItsLimitPasses(t *testing.T) {
	t.Parallel()

	// Act
	err := messaging.MessageLength(config.KindDiscord, strings.Repeat("a", 2000)).Check()
	// Assert
	if err != nil {
		t.Errorf("Check at the limit = %v, want nil", err)
	}
}
