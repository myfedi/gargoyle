import type React from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount } from "@/types/mastodon";

type AccountComboboxProps = {
  value: string;
  onValueChange: (value: string) => void;
  onSelect: (account: MastodonAccount) => void;
  onResolve: (query: string) => void;
  searchKnownAccounts: (query: string) => Promise<MastodonAccount[]>;
  isResolving?: boolean;
  placeholder?: string;
};

export function AccountCombobox({
  value,
  onValueChange,
  onSelect,
  onResolve,
  searchKnownAccounts,
  isResolving = false,
  placeholder = "Search accounts",
}: AccountComboboxProps) {
  const [results, setResults] = useState<MastodonAccount[]>([]);
  const [isSearching, setIsSearching] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const trimmedValue = value.trim();
  const canResolve = isResolvableRemoteQuery(trimmedValue);
  const searchIdRef = useRef(0);

  useEffect(() => {
    if (trimmedValue.length < 2) {
      setResults([]);
      setIsOpen(false);
      setError(null);
      return;
    }

    const searchId = searchIdRef.current + 1;
    searchIdRef.current = searchId;
    const timeout = window.setTimeout(() => {
      setIsSearching(true);
      setIsOpen(true);
      setError(null);

      searchKnownAccounts(trimmedValue)
        .then((accounts) => {
          if (searchIdRef.current === searchId) {
            setResults(accounts);
          }
        })
        .catch((caughtError: unknown) => {
          if (searchIdRef.current === searchId) {
            setError(caughtError instanceof Error ? caughtError.message : "Could not search accounts.");
          }
        })
        .finally(() => {
          if (searchIdRef.current === searchId) {
            setIsSearching(false);
          }
        });
    }, 250);

    return () => window.clearTimeout(timeout);
  }, [searchKnownAccounts, trimmedValue]);

  const showRemoteHint = useMemo(() => canResolve && results.length === 0 && !isSearching, [canResolve, isSearching, results.length]);

  function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (canResolve) {
      onResolve(normalizeRemoteQuery(trimmedValue));
    }
  }

  return (
    <div className="relative">
      <form className="flex flex-col gap-3 sm:flex-row" onSubmit={submit}>
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
          <Input
            value={value}
            onChange={(event) => {
              onValueChange(event.target.value);
              setIsOpen(true);
            }}
            onFocus={() => trimmedValue.length >= 2 && setIsOpen(true)}
            className="pl-9"
            placeholder={placeholder}
            aria-label="Search accounts"
            autoComplete="off"
          />
        </div>
        <Button type="submit" disabled={!canResolve || isResolving}>
          {isResolving ? "Looking up..." : "Lookup remote"}
        </Button>
      </form>

      {isOpen ? (
        <div className="absolute z-30 mt-2 w-full overflow-hidden rounded-lg border border-border bg-card shadow-lg">
          {isSearching ? (
            <div className="space-y-2 p-3" aria-label="Searching accounts">
              {[0, 1, 2].map((item) => <div key={item} className="h-12 animate-pulse rounded-md bg-secondary" />)}
            </div>
          ) : error ? (
            <p className="p-3 text-sm text-destructive">{error}</p>
          ) : results.length > 0 ? (
            <div className="max-h-80 overflow-auto p-1">
              {results.map((account) => (
                <button
                  key={account.id}
                  type="button"
                  className="block w-full rounded-md px-3 py-2 text-left hover:bg-accent hover:text-accent-foreground"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => {
                    onSelect(account);
                    setIsOpen(false);
                  }}
                >
                  <span className="block truncate text-sm font-medium">{account.display_name || account.username}</span>
                  <span className="block truncate text-xs text-muted-foreground">@{account.acct}</span>
                  {account.note ? <span className="mt-1 block truncate text-xs text-muted-foreground">{htmlToPlainText(account.note)}</span> : null}
                </button>
              ))}
            </div>
          ) : showRemoteHint ? (
            <button
              type="button"
              className="block w-full px-3 py-3 text-left text-sm hover:bg-accent hover:text-accent-foreground"
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => onResolve(normalizeRemoteQuery(trimmedValue))}
            >
              Look up {normalizeRemoteQuery(trimmedValue)}
            </button>
          ) : (
            <p className="p-3 text-sm text-muted-foreground">No known accounts.</p>
          )}
        </div>
      ) : null}
    </div>
  );
}

export function normalizeRemoteQuery(value: string) {
  const query = value.trim();
  if (query.startsWith("http://") || query.startsWith("https://") || query.startsWith("@")) {
    return query;
  }

  if (/^[^@\s]+@[^@\s]+$/.test(query)) {
    return `@${query}`;
  }

  return query;
}

function isResolvableRemoteQuery(value: string) {
  return value.startsWith("http://") || value.startsWith("https://") || /^@?[^@\s]+@[^@\s]+$/.test(value);
}
