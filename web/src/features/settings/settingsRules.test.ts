import { ruleFor } from './settingsRules.ts'

test.each([
  ['jira.base_url', '', true],
  ['jira.base_url', 'https://jira.example.com', true],
  ['jira.base_url', 'http://localhost:8080', true],
  ['jira.base_url', 'http://127.0.0.1:8080', true],
  ['jira.base_url', 'http://127.0.1.1:8080', true],
  ['jira.base_url', 'http://LocalHost:8080', true],
  ['jira.base_url', 'http://[::1]:8080', true],
  ['jira.base_url', 'http://[::ffff:127.0.0.1]:8080', true],
  ['jira.base_url', 'https://jira.example.com:99999', true],
  ['jira.base_url', 'https://', 'An address such as https://jira.example.com.'],
  ['jira.base_url', 'jira.example.com', 'An address such as https://jira.example.com.'],
  ['jira.base_url', 'ftp://jira.example.com', 'An address such as https://jira.example.com.'],
  [
    'jira.base_url',
    'http://jira.example.com',
    'Over http the token would cross the network in the clear: use https.',
  ],
  [
    'jira.base_url',
    'http://0.0.0.0:8080',
    'Over http the token would cross the network in the clear: use https.',
  ],
  [
    'jira.base_url',
    'http://[::ffff:10.0.0.1]:8080',
    'Over http the token would cross the network in the clear: use https.',
  ],
  [
    'jira.base_url',
    'https://ana@jira.example.com',
    'An address carries no login: the user and the token have fields of their own.',
  ],
  ['branch.template', '', true],
  ['branch.template', '{prefix}/{key}-{slug}', true],
  ['branch.template', '{prefix}/{slug}', 'Must contain {key}.'],
  ['timing.request_timeout', '', true],
  ['timing.request_timeout', '1m30s', true],
  ['timing.ci_interval', '1.5h', true],
  ['timing.ci_interval', '+20s', true],
  ['timing.ci_interval', '500\u00b5s', true],
  ['timing.ci_interval', '500\u03bcs', true],
  ['timing.ci_interval', '-20s', 'Must be more than zero.'],
  ['timing.ci_interval', '20', 'A duration such as 20s or 3m.'],
  ['timing.ci_interval', 'soon', 'A duration such as 20s or 3m.'],
  ['timing.ci_interval', '0s', 'Must be more than zero.'],
  ['commit.refs_trailer', '', true],
  ['commit.refs_trailer', 'Closes', true],
  ['commit.refs_trailer', ' Closes ', true],
  ['commit.refs_trailer', 'Signed\u00a0off', true],
  ['commit.refs_trailer', 'Closes \u0085', true],
  ['commit.refs_trailer', 'Closes:', 'A single word, without a colon.'],
  ['commit.refs_trailer', 'Fixed by', 'A single word, without a colon.'],
  ['commit.subject_limit', 0, true],
  ['commit.subject_limit', 50, true],
  ['commit.subject_limit', -1, 'Cannot be below zero.'],
  ['branch.slug_limit', -2, 'Cannot be below zero.'],
  ['ui.comments_shown', -1, 'Cannot be below zero.'],
])('%s refuses %j with %j', (name, value, said) => {
  // Arrange
  const rule = ruleFor(name)

  // Act
  const verdict = rule?.(value)

  // Assert
  expect(verdict).toBe(said)
})

test('a setting no rule checks has no rule', () => {
  // Act
  const rule = ruleFor('jira.project')

  // Assert
  expect(rule).toBeUndefined()
})
