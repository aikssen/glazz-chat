export default function Home() {
  return (
    <main className="grid min-h-dvh grid-rows-[auto_1fr_auto] bg-background text-foreground">
      <header className="flex h-14 items-center border-b border-border px-4">
        <span className="font-display text-xl font-semibold">Glazz</span>
        <span className="ml-auto text-sm text-muted-foreground">Foundation preview</span>
      </header>

      <section
        aria-labelledby="chat-heading"
        className="mx-auto flex w-full max-w-3xl flex-col justify-center px-4 py-10"
      >
        <div className="border-l-2 border-primary pl-5">
          <h1 id="chat-heading" className="font-display text-3xl font-semibold">
            What can I help you explore?
          </h1>
          <p className="mt-3 max-w-xl text-base leading-7 text-muted-foreground">
            The chat workflow will be implemented from the approved API and realtime contracts.
          </p>
        </div>
      </section>

      <footer className="sticky bottom-0 border-t border-border bg-background/95 p-4 backdrop-blur">
        <div className="mx-auto flex min-h-14 w-full max-w-3xl items-center rounded-lg border border-border bg-surface px-4 text-muted-foreground shadow-sm">
          Ask Glazz...
        </div>
      </footer>
    </main>
  );
}
