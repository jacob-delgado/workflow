// The Arrange-Act-Assert check the web's tests answer to, the e2e specs and
// the unit tests alike, as cmd/testshape is the Go tests' (CLAUDE.md, "Arrange,
// Act, Assert — marked in every test"): each test body marks its parts, in
// order, each on a line of its own between the body's statements; a flow of
// several Acts labels every step, while a single cycle carries no label, and
// an Arrange never does, since the test name is the label; every Assert
// reaches an expect, or a helper named for one, so it asserts something; and
// an Arrange or an Act checks no value, which is an Assert of a step of its
// own. An awaited expect on a locator stays allowed there: it waits for the
// page, as a step must before the next.

// keywords are the markers, as written after "// ".
const keywords = new Map([
  ['Arrange', 'arrange'],
  ['Act', 'act'],
  ['Assert', 'assert'],
  ['Act & Assert', 'actAndAssert'],
])

// grammar is where each marker leads from each state, or what it breaks: an
// optional Arrange, then one or more cycles, each an Act and its Assert or a
// single Act & Assert.
const grammar = {
  start: {
    arrange: 'arranged',
    act: 'acting',
    actAndAssert: 'asserted',
    assert: 'an Assert before any Act',
  },
  arranged: {
    act: 'acting',
    actAndAssert: 'asserted',
    arrange: 'a second Arrange',
    assert: 'an Assert before any Act',
  },
  acting: {
    assert: 'asserted',
    arrange: 'an Arrange between an Act and its Assert',
    act: "an Act before the last one's Assert",
    actAndAssert: "an Act & Assert before the last Act's Assert",
  },
  asserted: {
    act: 'acting',
    actAndAssert: 'asserted',
    arrange: "an Arrange after an Assert: setup for a later step belongs in that step's Act",
    assert: 'two Asserts in a row: an Assert follows its own Act',
  },
}

// unfinished is what a body that stops in a state leaves out.
const unfinished = {
  arranged: 'an Arrange with no Act after it',
  acting: 'an Act with no Assert after it',
}

const states = new Set(Object.keys(grammar))

// parseMarker reads a line comment written exactly as a marker: a keyword,
// then optionally ": " and a label.
function parseMarker(comment) {
  if (comment.type !== 'Line' || !comment.value.startsWith(' ')) {
    return undefined
  }

  const text = comment.value.slice(1).trimEnd()
  const colon = text.indexOf(':')
  const keyword = colon < 0 ? text : text.slice(0, colon)
  const kind = keywords.get(keyword)
  if (kind === undefined) {
    return undefined
  }

  const label = colon < 0 ? undefined : text.slice(colon + 1)
  if (label !== undefined && (!label.startsWith(' ') || label.trim() === '')) {
    return undefined
  }

  return { kind, labeled: label !== undefined, comment }
}

// looksLikeMarker reports whether a comment that is not a marker was meant to
// be one: a keyword in another case, spacing or comment style. Prose that
// merely starts with a keyword is not.
function looksLikeMarker(comment) {
  const loose = comment.value
    .split(/\s+/)
    .filter((word) => word !== '')
    .join(' ')
    .toLowerCase()
    .replaceAll(' and ', ' & ')
  const keyword = loose.split(':')[0].trim()

  return ['arrange', 'act', 'assert', 'act & assert'].includes(keyword)
}

// testBody is the body of a test — test(…), test.only(…) and the like, or a
// table's test.each(…)(…) — wherever after its name the test function sits,
// since Vitest takes a timeout or options after it and Playwright and Vitest
// take options before it. It is never the first argument: there Playwright's
// conditional test.skip(condition, reason) puts a condition, not a body. A
// hook, a describe or an extension has none.
function testBody(call) {
  if (!namesTest(call.callee)) {
    return undefined
  }

  const fn = call.arguments
    .slice(1)
    .find((argument) => ['ArrowFunctionExpression', 'FunctionExpression'].includes(argument.type))

  return fn?.body.type === 'BlockStatement' ? fn.body : undefined
}

// namesTest reports whether a callee is test, one of its modifiers, or the
// test a table's test.each(…) makes.
function namesTest(callee) {
  if (callee.type === 'Identifier') {
    return callee.name === 'test'
  }

  if (callee.type === 'CallExpression') {
    return isTestMember(callee.callee, ['each'])
  }

  return isTestMember(callee, ['only', 'skip', 'fixme', 'fail'])
}

// isTestMember reports whether a callee is test.<one of names>.
function isTestMember(callee, names) {
  return (
    callee.type === 'MemberExpression' &&
    callee.object.type === 'Identifier' &&
    callee.object.name === 'test' &&
    names.includes(callee.property.name)
  )
}

// expects reports whether a node calls expect, or a helper named for
// expecting, anywhere within it.
function expects(node) {
  if (node === null || typeof node !== 'object') {
    return false
  }

  if (node.type === 'CallExpression' && /^expect([A-Z]|$)/.test(calleeRoot(node.callee))) {
    return true
  }

  return Object.entries(node).some(
    ([key, child]) =>
      key !== 'parent' && (Array.isArray(child) ? child.some(expects) : expects(child)),
  )
}

// calleeRoot names what a call is made on: expect for expect.poll(…)(…).toBe,
// the function for a plain call.
function calleeRoot(callee) {
  let root = callee
  while (root.type === 'MemberExpression' || root.type === 'CallExpression') {
    root = root.type === 'MemberExpression' ? root.object : root.callee
  }

  return root.type === 'Identifier' ? root.name : ''
}

// readMarkers finds the markers among a body's comments, and the comments
// meant as markers but written wrongly.
function readMarkers(sourceCode, body) {
  const markers = []
  const malformed = []
  for (const comment of sourceCode.getCommentsInside(body)) {
    const marker = parseMarker(comment)
    if (marker !== undefined) {
      markers.push(marker)
    } else if (looksLikeMarker(comment)) {
      malformed.push(comment)
    }
  }

  return { markers, malformed }
}

// misplaced reports whether a marker sits anywhere but on a line of its own
// between a body's statements.
function misplaced(body, comment) {
  const line = comment.loc.start.line
  if (line === body.loc.start.line) {
    return true
  }

  return body.body.some(
    (statement) =>
      (comment.range[0] > statement.range[0] && comment.range[0] < statement.range[1]) ||
      statement.loc.start.line === line ||
      statement.loc.end.line === line,
  )
}

// sections divides a body's statements among its markers, and returns those
// before the first.
function sections(body, markers) {
  const owned = markers.map((marker) => ({ marker, statements: [] }))
  const leading = []
  for (const statement of body.body) {
    const owner = owned.findLast(({ marker }) => marker.comment.range[1] <= statement.range[0])
    if (owner === undefined) {
      leading.push(statement)
    } else {
      owner.statements.push(statement)
    }
  }

  return { owned, leading }
}

// orderProblem walks the markers through the grammar, and names the first out
// of place, or a body that stops short of its last Assert.
function orderProblem(markers) {
  let state = 'start'
  for (const marker of markers) {
    const next = grammar[state][marker.kind]
    if (!states.has(next)) {
      return { marker, problem: next }
    }
    state = next
  }

  const short = unfinished[state]

  return short === undefined ? undefined : { marker: markers.at(-1), problem: short }
}

// checkBody reports what a test body gets wrong, stopping at the first kind
// of problem that would leave the rest a guess.
function checkBody(context, call, body) {
  const { markers, malformed } = readMarkers(context.sourceCode, body)
  const wrong = [
    ...malformed.map((comment) => ({ loc: comment.loc, messageId: 'malformedMarker' })),
    ...markers
      .filter((marker) => misplaced(body, marker.comment))
      .map((marker) => ({ loc: marker.comment.loc, messageId: 'markerPlacement' })),
  ]
  if (wrong.length > 0) {
    wrong.forEach((problem) => context.report(problem))

    return
  }

  if (markers.length === 0) {
    context.report({ node: call, messageId: 'missingMarkers' })

    return
  }

  const { owned, leading } = sections(body, markers)
  if (leading.length > 0) {
    context.report({ node: leading[0], messageId: 'codeBeforeMarker' })

    return
  }

  const order = orderProblem(markers)
  if (order !== undefined) {
    context.report({
      loc: order.marker.comment.loc,
      messageId: 'markerOrder',
      data: { problem: order.problem },
    })

    return
  }

  checkSections(context, owned)
}

// checks reports whether a statement checks a value: an expect it does not
// await, which waits for nothing.
function checks(statement) {
  return (
    statement.type === 'ExpressionStatement' &&
    statement.expression.type === 'CallExpression' &&
    calleeRoot(statement.expression.callee) === 'expect'
  )
}

// labelProblem names what a marker's label gets wrong: a flow of several
// cycles labels every Act and Assert, and a single cycle or an Arrange carries
// none.
function labelProblem(marker, cycles) {
  if (marker.kind === 'arrange' || cycles === 1) {
    return marker.labeled ? 'labelUnexpected' : undefined
  }

  return marker.labeled ? undefined : 'labelRequired'
}

// checkSections reports labels where they do not belong or missing where they
// do, sections with nothing in them, Asserts that reach no expect, and a value
// checked outside an Assert.
function checkSections(context, owned) {
  const cycles = owned.filter(({ marker }) => ['act', 'actAndAssert'].includes(marker.kind)).length
  for (const { marker, statements } of owned) {
    const loc = marker.comment.loc
    const asserting = ['assert', 'actAndAssert'].includes(marker.kind)
    const checked = asserting ? undefined : statements.find(checks)
    const label = labelProblem(marker, cycles)
    if (label !== undefined) {
      context.report({ loc, messageId: label })
    } else if (statements.length === 0) {
      context.report({ loc, messageId: 'emptySection' })
    } else if (asserting && !statements.some(expects)) {
      context.report({ loc, messageId: 'assertWithoutExpect' })
    } else if (checked !== undefined) {
      context.report({ node: checked, messageId: 'checkOutsideAssert' })
    }
  }
}

export const arrangeActAssert = {
  meta: {
    type: 'problem',
    docs: {
      description: 'Mark each test body Arrange, Act and Assert, each Assert reaching an expect',
    },
    schema: [],
    messages: {
      missingMarkers:
        'Mark the test’s parts with // Arrange, // Act and // Assert, or // Act & Assert.',
      malformedMarker:
        'Write a marker as // Arrange, // Act, // Assert or // Act & Assert, optionally followed by ": a label".',
      markerPlacement: 'Put a marker on a line of its own between the test’s statements.',
      codeBeforeMarker: 'Code before the first marker belongs under an // Arrange.',
      markerOrder: 'The markers are out of order: {{problem}}.',
      labelRequired:
        'A flow of several steps labels every Act and Assert, as in // Act: open the preview.',
      labelUnexpected:
        'A single Act and Assert carries no label, and an Arrange never does: the test name is the label.',
      emptySection: 'This section holds nothing: drop its marker, or put its step under it.',
      assertWithoutExpect:
        'This Assert reaches no expect, nor a helper named for one, so it asserts nothing.',
      checkOutsideAssert:
        'A value checked before the Act is an Assert of a step of its own: label the flow, as in // Assert: nothing is sent yet.',
    },
  },
  create(context) {
    return {
      CallExpression(call) {
        const body = testBody(call)
        if (body !== undefined) {
          checkBody(context, call, body)
        }
      },
    }
  },
}
