import { wikiFromMarkdown } from './wikiFromMarkdown.ts'

// starBullet is the wiki bullet every Markdown bullet marker converts to; it
// is also the source form of the star-marked bullet, which must not be read
// as emphasis.
const starBullet = '* item'

// Twins of TestWikiFromMarkdown in internal/jira/wiki_test.go, case for case,
// so Preview shows the wiki markup Jira will be sent.
test.each([
  {
    name: 'plain prose is left alone',
    markdown: 'Tokens reach the log when the header is empty. See "quotes".',
    want: 'Tokens reach the log when the header is empty. See "quotes".',
  },
  {
    name: 'headings become h-levels',
    markdown: '# Title\n## Section',
    want: 'h1. Title\nh2. Section',
  },
  {
    name: 'bold and italic',
    markdown: 'This is **bold** and *italic* text.',
    want: 'This is *bold* and _italic_ text.',
  },
  {
    name: 'bold italic together',
    markdown: 'This is ***important*** text.',
    want: 'This is *_important_* text.',
  },
  {
    name: 'spaced asterisks are not emphasis',
    markdown: 'the array is 2 * 3 * 4 elements',
    want: 'the array is 2 * 3 * 4 elements',
  },
  {
    name: 'a spaced asterisk beside a real italic',
    markdown: 'a * b and *ital*',
    want: 'a * b and _ital_',
  },
  {
    name: 'intraword double underscores are literal',
    markdown: 'call a__b__c helper',
    want: 'call a__b__c helper',
  },
  {
    name: 'the underscore forms',
    markdown: 'A __bold__ and an _italic_ word.',
    want: 'A *bold* and an _italic_ word.',
  },
  {
    name: 'inline code becomes braces',
    markdown: 'Call `doThing()` first.',
    want: 'Call {{doThing()}} first.',
  },
  {
    name: 'a code span shields its contents',
    markdown: 'Literal `**stars**` but real **bold**.',
    want: 'Literal {{**stars**}} but real *bold*.',
  },
  {
    name: 'links',
    markdown: 'See [the docs](https://example.com/x) for more.',
    want: 'See [the docs|https://example.com/x] for more.',
  },
  {
    name: 'a link URL is copied verbatim',
    markdown: 'see [docs](https://ex.com/foo*bar*baz) now',
    want: 'see [docs|https://ex.com/foo*bar*baz] now',
  },
  {
    name: 'a link URL may hold balanced parentheses',
    markdown: '[a](https://en.wikipedia.org/wiki/Foo_(bar))',
    want: '[a|https://en.wikipedia.org/wiki/Foo_(bar)]',
  },
  {
    name: 'an image becomes Jira image markup',
    markdown: '![diagram](https://x/i.png)',
    want: '!https://x/i.png!',
  },
  {
    name: 'an image beside a link',
    markdown: '![img](a.png) and [link](b)',
    want: '!a.png! and [link|b]',
  },
  {
    name: 'a bullet list',
    markdown: '- first\n- second',
    want: '* first\n* second',
  },
  {
    name: 'a numbered list',
    markdown: '1. one\n2. two',
    want: '# one\n# two',
  },
  {
    name: 'a blockquote',
    markdown: '> a remark',
    want: 'bq. a remark',
  },
  {
    name: 'a blockquote with emphasis',
    markdown: '> a **bold** remark',
    want: 'bq. a *bold* remark',
  },
  {
    name: 'a star bullet',
    markdown: starBullet,
    want: starBullet,
  },
  {
    name: 'a plus bullet',
    markdown: '+ item',
    want: starBullet,
  },
  {
    name: 'strikethrough',
    markdown: '~~gone~~ now',
    want: '-gone- now',
  },
  {
    name: 'a fenced code block is bracketed and left verbatim',
    markdown: 'before\n```go\nx := **notBold**\n```\nafter',
    want: 'before\n{code:go}\nx := **notBold**\n{code}\nafter',
  },
  {
    name: 'a line ending in a carriage return',
    markdown: '# Title\r\nsome **bold**\r',
    want: 'h1. Title\r\nsome *bold*\r',
  },
  {
    name: 'an indented fence is not read as a fence',
    markdown: 'text\n    ```go\nmore **bold**',
    want: 'text\n    ```go\nmore *bold*',
  },
])('$name', ({ markdown, want }) => {
  // Act
  const got = wikiFromMarkdown(markdown)

  // Assert
  expect(got).toBe(want)
})
