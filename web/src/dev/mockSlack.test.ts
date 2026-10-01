import { beforeEach, vi } from 'vitest'

// The mockup's Slack refuses a link the server would, and forgets what a
// clean of everything removes.

// fresh is the mock modules as a page load finds them.
async function fresh() {
  const slack = await import('./mockSlack.ts')
  const localData = await import('./mockLocalData.ts')
  const { apiErrorMessage } = await import('@/api/apiError.ts')

  return { ...slack, ...localData, apiErrorMessage }
}

beforeEach(() => {
  vi.resetModules()
})

test.each([
  ['a team to a user', { owner: 'acme/control-plane', slack_id: 'U0ANA', not_on_slack: false }],
  ['a person to a group', { owner: 'ben', slack_id: 'S0POD', not_on_slack: false }],
  ['a person Slack does not list', { owner: 'ben', slack_id: 'U0NOBODY', not_on_slack: false }],
  ['a group Slack does not list', { owner: 'acme/web', slack_id: 'S0NONE', not_on_slack: false }],
  [
    'a member of another channel',
    { owner: 'ben', slack_id: 'U0ERIN', not_on_slack: false, channel: '#dev-workflow' },
  ],
  ['both a person and not on Slack', { owner: 'ben', slack_id: 'U0BEN', not_on_slack: true }],
])('the mockup refuses linking %s, as the server does', async (_, link) => {
  // Arrange
  const { mockLinkPerson, mockPeople } = await fresh()
  const before = mockPeople()

  // Act
  const linking = () => mockLinkPerson(link)

  // Assert
  expect(linking).toThrow()
  expect(mockPeople()).toEqual(before)
})

test('a refusal says why, as the server words it', async () => {
  // Arrange
  const { apiErrorMessage, mockLinkPerson } = await fresh()

  // Act
  const caught = (() => {
    try {
      mockLinkPerson({ owner: 'ben', slack_id: 'S0POD', not_on_slack: false })
    } catch (refused) {
      return refused
    }
  })()

  // Assert
  expect(apiErrorMessage(caught, 'fallback')).toBe(
    'a team links to a Slack user group, and a person to a Slack user',
  )
})

test('the mockup links a member of the channel named', async () => {
  // Arrange
  const { mockLinkPerson } = await fresh()

  // Act
  const people = mockLinkPerson({
    owner: 'ben',
    slack_id: 'U0ERIN',
    not_on_slack: false,
    channel: '#releases',
  })

  // Assert
  expect(people.owners.find((owner) => owner.owner === 'ben')).toEqual({
    owner: 'ben',
    kind: 'user',
    state: 'linked',
    slack: { id: 'U0ERIN', label: 'Erin Park' },
  })
})

test('each channel has its own members', async () => {
  // Arrange
  const { mockSlackMembers } = await fresh()

  // Act
  const releases = mockSlackMembers('#releases').entries.map((member) => member.label)

  // Assert
  expect(releases).toEqual(['Ana Souza', 'Erin Park'])
})

test('cleaning everything forgets the people and the groups', async () => {
  // Arrange
  const { mockCleanLocalData, mockPeople, mockRepoGroups } = await fresh()

  // Act
  mockCleanLocalData('all')

  // Assert
  expect(mockPeople().owners.every((owner) => owner.state === 'unlinked')).toBe(true)
  expect(mockRepoGroups().groups).toEqual([])
})
