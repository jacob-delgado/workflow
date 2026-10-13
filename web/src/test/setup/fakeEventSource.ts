type Listener = (event: MessageEvent) => void

// A controllable stand-in for the browser EventSource jsdom does not provide.
// test-setup installs it as the global; a test reaches the latest instance to
// push events and assert what the stream does with them.
export class FakeEventSource {
  static instances: FakeEventSource[] = []
  static readonly CLOSED = 2

  readonly url: string
  closed = false
  // The connection's state as EventSource reports it; only refuse changes it.
  readyState = 0
  private readonly listeners = new Map<string, Listener[]>()

  constructor(url: string | URL) {
    this.url = url.toString()
    FakeEventSource.instances.push(this)
  }

  addEventListener(type: string, listener: Listener): void {
    const list = this.listeners.get(type) ?? []
    list.push(listener)
    this.listeners.set(type, list)
  }

  removeEventListener(): void {
    // The hook closes the source rather than removing listeners; nothing to do.
  }

  close(): void {
    this.closed = true
  }

  emit(type: string, data: string): void {
    for (const listener of this.listeners.get(type) ?? []) {
      listener(new MessageEvent(type, { data }))
    }
  }

  // refuse stands for a server that answers with something other than a
  // stream, as it does (404) for a view it does not have: the browser closes
  // the source for good, without retrying, and fires one error.
  refuse(): void {
    this.readyState = FakeEventSource.CLOSED
    this.emit('error', '')
  }

  static reset(): void {
    FakeEventSource.instances = []
  }

  static latest(): FakeEventSource {
    const instance = FakeEventSource.instances.at(-1)
    if (!instance) {
      throw new Error('no EventSource was opened')
    }

    return instance
  }
}
