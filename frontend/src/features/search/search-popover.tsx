import { useCallback, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { AccountCombobox, normalizeRemoteQuery } from "@/features/accounts/account-combobox";
import { EmptyState } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import { htmlToPlainText } from "@/lib/text";
import type { MastodonAccount } from "@/types/mastodon";

type SearchPopoverProps = {
  onClose: () => void;
};

export function SearchPopover({ onClose }: SearchPopoverProps) {
  const { session } = useAuth();
  const [query, setQuery] = useState("");
  const [remoteResults, setRemoteResults] = useState<MastodonAccount[]>([]);
  const [isResolving, setIsResolving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const searchKnownAccounts = useCallback(async (searchQuery: string) => {
    if (!api) return [];
    return api.searchKnownAccounts(searchQuery);
  }, [api]);

  function openAccount(account: MastodonAccount) {
    window.location.hash = accountHref(account.id).replace(/^\//, "");
    onClose();
  }

  async function resolveAccount(searchQuery: string) {
    if (!api || !searchQuery.trim()) return;
    setIsResolving(true);
    setError(null);

    try {
      const search = await api.searchAccounts(normalizeRemoteQuery(searchQuery));
      setRemoteResults(search.accounts);
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not look up account.");
    } finally {
      setIsResolving(false);
    }
  }

  return (
    <div className="border-t border-border bg-background/98 shadow-sm">
      <div className="mx-auto max-w-3xl px-4 py-4 md:px-6">
        <div className="rounded-lg border border-border bg-card p-4 shadow-lg">
          <AccountCombobox
            value={query}
            onValueChange={(value) => {
              setQuery(value);
              setRemoteResults([]);
            }}
            searchKnownAccounts={searchKnownAccounts}
            isResolving={isResolving}
            placeholder="Search for people"
            onSelect={openAccount}
            onResolve={(searchQuery) => void resolveAccount(searchQuery)}
          />

          {error ? <p className="mt-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}

          {remoteResults.length > 0 ? (
            <div className="mt-4 divide-y divide-border rounded-md border border-border bg-background">
              {remoteResults.map((account) => (
                <button
                  key={account.id}
                  type="button"
                  className="block w-full px-3 py-3 text-left hover:bg-accent hover:text-accent-foreground"
                  onClick={() => openAccount(account)}
                >
                  <span className="block truncate text-sm font-medium">{account.display_name || account.username}</span>
                  <span className="block truncate text-xs text-muted-foreground">@{account.acct}</span>
                  {account.note ? <span className="mt-1 block truncate text-xs text-muted-foreground">{htmlToPlainText(account.note)}</span> : null}
                </button>
              ))}
            </div>
          ) : query.trim().length === 0 ? (
            <div className="mt-4">
              <EmptyState title="Find people" description="Search a local handle, remote handle, or profile URL." />
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
