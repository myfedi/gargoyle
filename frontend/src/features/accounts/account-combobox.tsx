import { useEffect, useMemo, useRef, useState } from "react";
import { Search } from "lucide-react";

import { Input } from "@/components/ui/input";
import { ApiError } from "@/lib/api";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount } from "@/types/mastodon";

type AccountComboboxProps = {
  value: string;
  onValueChange: (value: string) => void;
  onSelect: (account: MastodonAccount) => void;
  onResolve: (query: string) => Promise<MastodonAccount[] | void> | MastodonAccount[] | void;
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
  const [isRemoteLookupRunning, setIsRemoteLookupRunning] = useState(false);
  const [isOpen, setIsOpen] = useState(false);
  const [localError, setLocalError] = useState<string | null>(null);
  const [remoteError, setRemoteError] = useState<string | null>(null);
  const trimmedValue = value.trim();
  const canResolve = isResolvableRemoteQuery(trimmedValue);
  const searchIdRef = useRef(0);
  const resolvedQueryRef = useRef<string | null>(null);

  useEffect(() => {
    if (trimmedValue.length < 2) {
      setResults([]);
      setIsOpen(false);
      setLocalError(null);
      setRemoteError(null);
      setIsRemoteLookupRunning(false);
      return;
    }

    const searchId = searchIdRef.current + 1;
    searchIdRef.current = searchId;
    const timeout = globalThis.setTimeout(() => {
      setIsSearching(true);
      setIsOpen(true);
      setLocalError(null);
      setRemoteError(null);

      searchKnownAccounts(knownAccountSearchQuery(trimmedValue))
        .then((accounts) => {
          if (searchIdRef.current !== searchId) return;

          setResults(accounts);
          const normalizedQuery = normalizeRemoteQuery(trimmedValue);
          if (accounts.length === 0 && isResolvableRemoteQuery(trimmedValue) && resolvedQueryRef.current !== normalizedQuery) {
            resolvedQueryRef.current = normalizedQuery;
            setIsRemoteLookupRunning(true);
            Promise.resolve(onResolve(normalizedQuery))
              .then((resolvedAccounts) => {
                if (searchIdRef.current === searchId && Array.isArray(resolvedAccounts)) {
                  setResults(resolvedAccounts);
                  if (resolvedAccounts.length === 0) {
                    setRemoteError("No remote account found.");
                  }
                }
              })
              .catch((caughtError: unknown) => {
                if (searchIdRef.current === searchId) {
                  setRemoteError(remoteLookupMessage(caughtError, trimmedValue));
                }
              })
              .finally(() => {
                if (searchIdRef.current === searchId) {
                  setIsRemoteLookupRunning(false);
                }
              });
          }
        })
        .catch(() => {
          if (searchIdRef.current === searchId) {
            setLocalError("Could not search local accounts.");
            setResults([]);
          }
        })
        .finally(() => {
          if (searchIdRef.current === searchId) {
            setIsSearching(false);
          }
        });
    }, 250);

    return () => globalThis.clearTimeout(timeout);
  }, [onResolve, searchKnownAccounts, trimmedValue]);

  const normalizedValue = normalizeRemoteQuery(trimmedValue);
  const hasTriedRemote = resolvedQueryRef.current === normalizedValue;
  const isLookingUpRemote = isResolving || isRemoteLookupRunning;
  const showRemoteHint = useMemo(
    () => canResolve && results.length === 0 && !isSearching && !isLookingUpRemote && !hasTriedRemote,
    [canResolve, hasTriedRemote, isLookingUpRemote, isSearching, results.length],
  );

  return (
    <div className="relative">
      <div className="relative">
        <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" aria-hidden="true" />
        <Input
          value={value}
          onChange={(event) => {
            onValueChange(event.target.value);
            resolvedQueryRef.current = null;
            setRemoteError(null);
            setLocalError(null);
            setIsOpen(true);
          }}
          onFocus={() => trimmedValue.length >= 2 && setIsOpen(true)}
          className="pl-9"
          placeholder={placeholder}
          aria-label="Search accounts"
          autoComplete="off"
        />
      </div>

      {isOpen ? (
        <div className="absolute z-30 mt-2 w-full overflow-hidden rounded-lg border border-border bg-card shadow-lg">
          {isSearching ? (
            <div className="space-y-2 p-3" aria-label="Searching accounts">
              {[0, 1, 2].map((item) => <div key={item} className="h-12 animate-pulse rounded-md bg-secondary" />)}
            </div>
          ) : results.length > 0 ? (
            <div className="max-h-80 overflow-auto p-1">
              {results.map((account) => (
                <button
                  key={account.id}
                  type="button"
                  className="flex w-full gap-3 rounded-md px-3 py-2 text-left hover:bg-accent hover:text-accent-foreground"
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => {
                    onSelect(account);
                    setIsOpen(false);
                  }}
                >
                  <AccountResultAvatar account={account} />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{account.display_name || account.username}</span>
                    <span className="block truncate text-xs text-muted-foreground">@{account.acct}</span>
                    {account.note ? <span className="mt-1 block truncate text-xs text-muted-foreground">{htmlToPlainText(account.note)}</span> : null}
                  </span>
                </button>
              ))}
            </div>
          ) : isLookingUpRemote ? (
            <div className="space-y-2 p-3" aria-label="Looking up remote account">
              {[0, 1].map((item) => <div key={item} className="h-12 animate-pulse rounded-md bg-secondary" />)}
            </div>
          ) : remoteError ? (
            <p className="p-3 text-sm text-destructive">{remoteError}</p>
          ) : localError ? (
            <p className="p-3 text-sm text-destructive">{localError}</p>
          ) : showRemoteHint ? (
            <p className="p-3 text-sm text-muted-foreground">Looking up remote account...</p>
          ) : canResolve && hasTriedRemote ? (
            <p className="p-3 text-sm text-muted-foreground">No remote account found.</p>
          ) : (
            <p className="p-3 text-sm text-muted-foreground">No known accounts.</p>
          )}
        </div>
      ) : null}
    </div>
  );
}

function AccountResultAvatar({ account }: { account: MastodonAccount }) {
  const avatar = account.avatar_static || account.avatar;

  if (avatar) {
    return <img className="size-10 shrink-0 rounded-full border border-border object-cover" src={avatar} alt="" aria-hidden="true" />;
  }

  return (
    <span className="flex size-10 shrink-0 items-center justify-center rounded-full border border-border bg-secondary text-sm font-semibold uppercase text-secondary-foreground" aria-hidden="true">
      {accountInitial(account)}
    </span>
  );
}

function accountInitial(account: MastodonAccount) {
  const label = account.display_name || account.username || account.acct || "?";
  return label.trim().slice(0, 1) || "?";
}

export function knownAccountSearchQuery(value: string) {
  const query = value.trim();
  const profileHandle = handleFromProfileUrl(query);
  if (profileHandle) {
    return profileHandle;
  }

  if (query.endsWith("@")) {
    return query.slice(0, -1);
  }

  return query;
}

export function normalizeRemoteQuery(value: string) {
  const query = value.trim();
  const profileHandle = handleFromProfileUrl(query);
  if (profileHandle) {
    return `@${profileHandle}`;
  }
  if (query.startsWith("http://") || query.startsWith("https://") || query.startsWith("@")) {
    return query;
  }

  if (/^[^@\s]+@[^@\s]+$/.test(query)) {
    return `@${query}`;
  }

  return query;
}

function remoteLookupMessage(error: unknown, query: string) {
  const message = error instanceof Error ? error.message : "Could not look up that remote account.";
  if (message.toLowerCase().includes("private address")) {
    return "That URL points to a private or local address. Try the account handle instead.";
  }
  if (error instanceof ApiError && error.status === 404) {
    return "No remote account found.";
  }
  if (isProfileUrl(query)) {
    return "Could not look up that profile URL.";
  }
  return "Could not look up that remote account.";
}

function isResolvableRemoteQuery(value: string) {
  return value.startsWith("http://") || value.startsWith("https://") || /^@?[^@\s]+@[^@\s]+$/.test(value);
}

function isProfileUrl(value: string) {
  try {
    const url = new URL(value);
    return /^\/(?:@|users\/)[^/]+\/?$/.test(url.pathname);
  } catch {
    return false;
  }
}

function handleFromProfileUrl(value: string) {
  try {
    const url = new URL(value);
    const remoteHandle = url.pathname.match(/^\/@([^/@\s]+)@([^/@\s]+)\/?$/);
    if (remoteHandle) {
      return `${remoteHandle[1]}@${remoteHandle[2]}`;
    }
    const localHandle = url.pathname.match(/^\/(?:@|users\/)([^/@\s]+)\/?$/);
    if (localHandle) {
      return `${localHandle[1]}@${url.hostname}`;
    }
    return null;
  } catch {
    return null;
  }
}
