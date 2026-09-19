export default function App() {
  return (
    <main className="mx-auto flex min-h-dvh max-w-2xl flex-col justify-center gap-4 px-4">
      <h1 className="text-3xl font-semibold tracking-tight">workflow</h1>
      <p className="text-slate-600 dark:text-slate-400">
        The web cockpit for your Jira, Git, and Slack workflow. Serve it from the terminal with{' '}
        <code className="rounded bg-slate-200 px-1 py-0.5 font-mono text-sm dark:bg-slate-800">
          workflow --web
        </code>
        .
      </p>
    </main>
  )
}
