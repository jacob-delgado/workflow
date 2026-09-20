export default function App() {
  return (
    <main className="mx-auto flex min-h-dvh max-w-2xl flex-col justify-center gap-4 px-4">
      <h1 className="text-3xl font-semibold tracking-tight text-foreground">workflow</h1>
      <p className="text-muted-foreground">
        The web cockpit for your Jira, Git, and Slack workflow. Serve it from the terminal with{' '}
        <code className="rounded-sm bg-muted px-1 py-0.5 font-mono text-sm text-foreground">
          workflow --web
        </code>
        .
      </p>
    </main>
  )
}
