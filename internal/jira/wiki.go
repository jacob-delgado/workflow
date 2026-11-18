// Copyright 2026 Jacob Delgado
// SPDX-License-Identifier: Apache-2.0

package jira

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// boldSentinel stands in for wiki-bold's single asterisk while the emphasis
// rewrites run, so the italic pass does not read a just-made bold as an italic.
// A NUL byte, which comment text does not carry.
const boldSentinel = "\x00"

// markdownHeading matches a Markdown heading line: its run of #s, then its text.
var markdownHeading = regexp.MustCompile(`^(#{1,6}) (.*)$`)

// unorderedItem matches a Markdown bulleted list item, capturing its text.
var unorderedItem = regexp.MustCompile(`^[-*+] (.*)$`)

// orderedItem matches a Markdown numbered list item, capturing its text.
var orderedItem = regexp.MustCompile(`^\d+\. (.*)$`)

// inlineCodeSpan matches a `code` span, capturing what is between the backticks.
var inlineCodeSpan = regexp.MustCompile("`([^`]+)`")

// linkOrImage matches a link, or — when the "!" group is present — an image. The
// URL group allows one level of balanced parentheses, for destinations like a
// Wikipedia "..._(disambiguation)" page.
var linkOrImage = regexp.MustCompile(`(!?)\[([^\]]*)\]\(([^()]*(?:\([^()]*\)[^()]*)*)\)`)

// strikethroughSpan matches ~~struck~~ text.
var strikethroughSpan = regexp.MustCompile(`~~(.+?)~~`)

// boldItalicSpan matches ***bold italic*** text.
var boldItalicSpan = regexp.MustCompile(`\*\*\*(.+?)\*\*\*`)

// starBoldSpan matches **bold** text.
var starBoldSpan = regexp.MustCompile(`\*\*(.+?)\*\*`)

// underscoreBoldSpan matches __bold__ only when the delimiters are flanked by a
// non-word character (or the string's edge), so an intraword double underscore —
// a dunder identifier — is left literal. The flanking characters are captured
// and re-emitted around the converted span.
var underscoreBoldSpan = regexp.MustCompile(`(^|[^\w])__(.+?)__([^\w]|$)`)

// starItalicSpan matches *italic* only when the content neither opens nor closes
// with whitespace, so a bare asterisk between spaces — multiplication, a glob —
// is not read as emphasis.
var starItalicSpan = regexp.MustCompile(`\*([^\s*](?:[^*]*?[^\s*])?)\*`)

// WikiFromMarkdown rewrites the Markdown a comment was written in as the wiki
// markup Jira renders: headings, emphasis, inline and fenced code, links,
// images, and lists. Text with no Markdown comes back unchanged, so it is safe
// on plain prose — a bare asterisk between spaces or an underscore inside a word
// is left alone rather than read as emphasis. A code block the comment leaves
// open is closed at its end, as Markdown ends one with the document.
//
// These rules are written again in
// web/src/features/issues/wiki/wikiFromMarkdown.ts, for the web's Preview, and
// both copies answer to the one case file testdata/wiki_from_markdown.json, so
// a change to either alone fails its own tests.
func WikiFromMarkdown(md string) string {
	lines := strings.Split(md, "\n")
	out := make([]string, 0, len(lines))
	block := wikiBlock{}

	for _, text := range lines {
		var converted string

		block, converted = block.line(text)
		out = append(out, converted)
	}

	out = append(out, block.end()...)

	return strings.Join(out, "\n")
}

// codeClose is the wiki markup that ends a code block.
const codeClose = "{code}"

// blockStage is whether a line is read as prose or inside a fenced code block.
// It has two values, so it is read by comparison.
type blockStage int

const (
	// blockProse is a line outside any code block, converted.
	blockProse blockStage = iota
	// blockCode is a line inside a fenced code block, kept as written.
	blockCode
)

// wikiBlock is where a comment's conversion stands as it reaches a line: in
// prose, or inside the code block its opener began.
type wikiBlock struct {
	stage  blockStage
	opener codeFence
}

// line converts one line read in the block, and answers the block the next
// line is read in: a fence in prose opens a code block, and inside one only a
// fence that closes its opener ends it.
func (b wikiBlock) line(text string) (wikiBlock, string) {
	fence, isFence := readFence(text)

	if b.stage == blockCode {
		if isFence && fence.closes(b.opener) {
			return wikiBlock{stage: blockProse}, codeClose
		}

		return b, text
	}

	if isFence {
		return wikiBlock{stage: blockCode, opener: fence}, fence.bracket()
	}

	return b, convertLine(text)
}

// end is what the block writes once the comment's lines run out: a code block
// still open gets its {code}, as Markdown ends one with the document, and
// prose writes nothing.
func (b wikiBlock) end() []string {
	if b.stage == blockCode {
		return []string{codeClose}
	}

	return nil
}

// codeFence is a line that opens or closes a fenced code block: the marker
// it runs, a backtick or a tilde, how many it runs to, and the info string
// after them, which names the block's language.
type codeFence struct {
	marker rune
	length int
	info   string
}

// closes reports whether the fence, read inside the block opener opened,
// closes it: only a bare fence of the opener's marker, at least as long, does,
// so a block that quotes a ```go line, a shorter fence or the other marker's
// keeps it.
func (f codeFence) closes(opener codeFence) bool {
	return f.marker == opener.marker && f.info == "" && f.length >= opener.length
}

// bracket is the wiki markup that opens a block at the fence: {code:lang}
// when it names a language, {code} otherwise.
func (f codeFence) bracket() string {
	if f.info == "" {
		return "{code}"
	}

	return "{code:" + f.info + "}"
}

// maxFenceIndent is how far a code fence may be indented before Markdown reads
// it as indented content rather than a fence, so a deeper indent must not toggle
// the fence state and swallow the rest of the comment.
const maxFenceIndent = 3

// byteOrderMark is U+FEFF, which JavaScript's trim takes as white space and
// Go's does not.
const byteOrderMark = '\uFEFF'

// trimFence is text without the white space around it, as both converters read
// it: Unicode's, with the next-line character Go's TrimSpace takes, and the byte
// order mark JavaScript's trim takes, so Preview brackets a fence as this does.
func trimFence(text string) string {
	return strings.TrimFunc(text, func(r rune) bool { return unicode.IsSpace(r) || r == byteOrderMark })
}

// fenceMarkers are the characters a code fence runs: backticks or tildes.
const fenceMarkers = "`~"

// minFenceLength is the fewest markers a code fence runs to.
const minFenceLength = 3

// readFence reads a ``` or ~~~ fence line, reporting false for any other line.
func readFence(line string) (codeFence, bool) {
	if len(line)-len(strings.TrimLeft(line, " ")) > maxFenceIndent {
		return codeFence{}, false
	}

	trimmed := trimFence(line)

	for _, marker := range fenceMarkers {
		info := strings.TrimLeft(trimmed, string(marker))
		if length := len(trimmed) - len(info); length >= minFenceLength {
			return codeFence{marker: marker, length: length, info: trimFence(info)}, true
		}
	}

	return codeFence{}, false
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
	if match := markdownHeading.FindStringSubmatch(line); match != nil {
		return "h" + strconv.Itoa(len(match[1])) + ". ", match[2]
	}

	if rest, ok := strings.CutPrefix(line, "> "); ok {
		return "bq. ", rest
	}

	if match := unorderedItem.FindStringSubmatch(line); match != nil {
		return "* ", match[1]
	}

	if match := orderedItem.FindStringSubmatch(line); match != nil {
		return "# ", match[1]
	}

	return "", line
}

// convertInline rewrites the emphasis, links and inline code in a run of text,
// leaving the content of a code span untouched by the emphasis rewrites.
func convertInline(text string) string {
	var out strings.Builder

	last := 0
	for _, span := range inlineCodeSpan.FindAllStringSubmatchIndex(text, -1) {
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
	for _, span := range linkOrImage.FindAllStringSubmatchIndex(text, -1) {
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
	text = strikethroughSpan.ReplaceAllString(text, "-${1}-")
	text = boldItalicSpan.ReplaceAllString(text, boldSentinel+"_${1}_"+boldSentinel)
	text = starBoldSpan.ReplaceAllString(text, boldSentinel+"${1}"+boldSentinel)
	text = underscoreBoldSpan.ReplaceAllString(text, "${1}"+boldSentinel+"${2}"+boldSentinel+"${3}")
	text = starItalicSpan.ReplaceAllString(text, "_${1}_")

	return strings.ReplaceAll(text, boldSentinel, "*")
}
