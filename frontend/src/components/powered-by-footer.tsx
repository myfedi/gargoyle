export function PoweredByFooter() {
  return (
    <footer className="mx-auto w-full max-w-6xl px-4 pb-6 pt-2 text-center text-xs text-muted-foreground md:px-6">
      powered by{" "}
      <a
        href="https://github.com/myfedi/gargoyle"
        target="_blank"
        rel="noreferrer"
        className="font-medium text-foreground underline decoration-border underline-offset-4 transition-colors hover:text-primary hover:decoration-primary"
      >
        gargoyle
      </a>
    </footer>
  );
}
