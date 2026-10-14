import type { ReactNode } from 'react'
import { CSPProvider as Base } from '@base-ui/react/csp-provider'

// CSPProvider keeps every Base UI part under it from writing a <style>
// element, which the page's content policy (default-src 'self', no nonce)
// refuses. Trade-off TRADE-50: only ScrollArea and Select write one, to hide a
// native scrollbar, and the page uses neither, keeping the native <select>.
export function CSPProvider({ children }: { children: ReactNode }) {
  return <Base disableStyleElements>{children}</Base>
}
