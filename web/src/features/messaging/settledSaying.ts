import type { QueuedAnnouncement } from '@/api/generated/types.gen.ts'
import { withWarning } from './withWarning.ts'

// SettledState is where a held announcement stands once it neither waits nor
// is being posted.
export type SettledState = Exclude<QueuedAnnouncement['state'], 'waiting' | 'announcing'>

// settled says whether a held announcement's state is one it has settled in.
export function settled(state: QueuedAnnouncement['state'] | undefined): state is SettledState {
  return state !== undefined && state !== 'waiting' && state !== 'announcing'
}

// settledSaying is what a held announcement settled in state says, to where,
// wherever it is shown: announced once CI passed, with the server's warning
// when the store could not confirm it; stopped or dropped, that it was not
// announced; and posted elsewhere, that it was not posted here; each with the
// server's reason, which for one stopped says whether Stop waiting or another
// ask that replaced it stopped it, else what became of it.
export function settledSaying(
  state: SettledState,
  hold: { reason?: string | undefined; warning?: string | undefined },
  where: string,
): string {
  switch (state) {
    case 'announced':
      return withWarning(`Announced to ${where} once CI passed.`, hold.warning)
    case 'stopped':
    case 'dropped':
      return `Not announced: ${hold.reason ?? `it was ${state}`}.`
    case 'elsewhere':
      return `Not posted here: ${hold.reason ?? 'it was announced from another session'}.`
  }
}
