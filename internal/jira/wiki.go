// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"regexp"
	"strconv"
	"strings"
)

// boldSentinel stands in for wiki-bold's single asterisk while the emphasis
// rewrites run, so the italic pass does not read a just-made bold as an italic.
// A caret-feed control byte, which comment text does not carry.
const boldSentinel = "\x00"

// WikiFromMarkdown rewrites the Markdown a comment was written in as the wiki
// markup Jira renders: headings, emphasis, inline and fenced code, links,
// images, and lists. Text with no Markdown comes back unchanged, so it is safe
// on plain prose — a bare asterisk between spaces or an underscore inside a word
// is left alone rather than read as emphasis.
func WikiFromMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	inFence := false

	for _, line := range lines {
		if fence, ok := fenceLine(line); ok {
			out = append(out, fence)
			inFence = !inFence

			continue
		}

		if inFence {
			out = append(out, line)

			continue
		}

		out = append(out, convertLine(line))
	}

	return strings.Join(out, "\n")
}

// maxFenceIndent is how far a code fence may be indented before Markdown reads
// it as indented content rather than a fence, so a deeper indent must not toggle
// the fence state and swallow the rest of the comment.
const maxFenceIndent = 3

// fenceLine turns a ``` fence into its wiki bracket: {code:lang} when it opens
// with a language, {code} otherwise. It reports whether the line was a fence.
func fenceLine(line string) (string, bool) {
	if len(line)-len(strings.TrimLeft(line, " ")) > maxFenceIndent {
		return "", false
	}

	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "```") {
		return "", false
	}

	if lang := strings.TrimSpace(strings.TrimPrefix(trimmed, "```")); lang != "" {
		return "{code:" + lang + "}", true
	}

	return "{code}", true
}

// convertLine rewrites one line outside a code block: its block marker, then the
// emphasis, code and links in its text.
func convertLine(line string) string {
	prefix, content := blockPrefix(line)

	return prefix + convertInline(content)
}

// blockPrefix reads a line's block marker — a heading, a blockquote, or a list
// item — returning the wiki prefix and the text after it, or no prefix and the
// line unchanged.
func blockPrefix(line string) (string, string) {
	if match := headingRE().FindStringSubmatch(line); match != nil {
		return "h" + strconv.Itoa(len(match[1])) + ". ", match[2]
	}

	if rest, ok := strings.CutPrefix(line, "> "); ok {
		return "bq. ", rest
	}

	if match := unorderedRE().FindStringSubmatch(line); match != nil {
		return "* ", match[1]
	}

	if match := orderedRE().FindStringSubmatch(line); match != nil {
		return "# ", match[1]
	}

	return "", line
}

// convertInline rewrites the emphasis, links and inline code in a run of text,
// leaving the content of a code span untouched by the emphasis rewrites.
func convertInline(text string) string {
	var out strings.Builder

	last := 0
	for _, span := range inlineCodeRE().FindAllStringSubmatchIndex(text, -1) {
		out.WriteString(convertEmphasis(text[last:span[0]]))
		out.WriteString("{{")
		out.WriteString(text[span[2]:span[3]])
		out.WriteString("}}")

		last = span[1]
	}

	out.WriteString(convertEmphasis(text[last:]))

	return out.String()
}

// convertEmphasis rewrites the links, images and emphasis in a run of text that
// holds no code span. A link's URL and an image's source are copied verbatim, so
// an emphasis marker or a closing parenthesis inside the URL is not read as
// markup; only the text around a link and the link's own label are emphasized.
func convertEmphasis(text string) string {
	var out strings.Builder

	last := 0
	for _, span := range linkRE().FindAllStringSubmatchIndex(text, -1) {
		out.WriteString(emphasize(text[last:span[0]]))
		writeLinkOrImage(&out, text, span)

		last = span[1]
	}

	out.WriteString(emphasize(text[last:]))

	return out.String()
}

// writeLinkOrImage writes the wiki form of a matched link or image: an image (a
// "!"-prefixed link) becomes Jira's !src!, and a link its [label|url] with the
// label — but never the URL — emphasized.
func writeLinkOrImage(out *strings.Builder, text string, span []int) {
	url := text[span[6]:span[7]]
	if text[span[2]:span[3]] == "!" {
		out.WriteString("!")
		out.WriteString(url)
		out.WriteString("!")

		return
	}

	out.WriteString("[")
	out.WriteString(emphasize(text[span[4]:span[5]]))
	out.WriteString("|")
	out.WriteString(url)
	out.WriteString("]")
}

// emphasize rewrites strikethrough and bold/italic in text with no link or code
// span. Bold is parked on a sentinel first so the italic pass does not mistake a
// fresh single asterisk for an italic of its own.
func emphasize(text string) string {
	text = strikeRE().ReplaceAllString(text, "-${1}-")
	text = boldItalicRE().ReplaceAllString(text, boldSentinel+"_${1}_"+boldSentinel)
	text = boldStarRE().ReplaceAllString(text, boldSentinel+"${1}"+boldSentinel)
	text = boldUnderRE().ReplaceAllString(text, "${1}"+boldSentinel+"${2}"+boldSentinel+"${3}")
	text = italicStarRE().ReplaceAllString(text, "_${1}_")

	return strings.ReplaceAll(text, boldSentinel, "*")
}

func headingRE() *regexp.Regexp    { return regexp.MustCompile(`^(#{1,6}) (.*)$`) }
func unorderedRE() *regexp.Regexp  { return regexp.MustCompile(`^[-*+] (.*)$`) }
func orderedRE() *regexp.Regexp    { return regexp.MustCompile(`^\d+\. (.*)$`) }
func inlineCodeRE() *regexp.Regexp { return regexp.MustCompile("`([^`]+)`") }
func strikeRE() *regexp.Regexp     { return regexp.MustCompile(`~~(.+?)~~`) }
func boldItalicRE() *regexp.Regexp { return regexp.MustCompile(`\*\*\*(.+?)\*\*\*`) }
func boldStarRE() *regexp.Regexp   { return regexp.MustCompile(`\*\*(.+?)\*\*`) }

// linkRE matches a link, or — when the "!" group is present — an image. The URL
// group allows one level of balanced parentheses, for destinations like a
// Wikipedia "..._(disambiguation)" page.
func linkRE() *regexp.Regexp {
	return regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)
}

// boldUnderRE matches __bold__ only when the delimiters are flanked by a
// non-word character (or the string's edge), so an intraword double underscore —
// a dunder identifier — is left literal. The flanking characters are captured
// and re-emitted around the converted span.
func boldUnderRE() *regexp.Regexp { return regexp.MustCompile(`(^|[^\w])__(.+?)__([^\w]|$)`) }

// italicStarRE matches *italic* only when the content neither opens nor closes
// with whitespace, so a bare asterisk between spaces — multiplication, a glob —
// is not read as emphasis.
func italicStarRE() *regexp.Regexp { return regexp.MustCompile(`\*([^\s*](?:[^*]*?[^\s*])?)\*`) }
