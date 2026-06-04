import { useCallback, useMemo, useState } from "react";
import { X } from "lucide-react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { AccountCombobox, knownAccountSearchQuery, normalizeRemoteQuery } from "@/features/accounts/account-combobox";
import { EmptyState } from "@/features/shared";
import { createMastodonApi } from "@/lib/mastodon-api";
import { accountHref } from "@/lib/routes";
import type { MastodonAccount } from "@/types/mastodon";

type SearchPopoverProps = {
  onClose: () => void;
};

export function SearchPopover({ onClose }: SearchPopoverProps) {
  const { session } = useAuth();
  const [query, setQuery] = useState("");
  const [isResolving, setIsResolving] = useState(false);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);

  const searchKnownAccounts = useCallback(async (searchQuery: string) => {
    if (!api) return [];
    return api.searchKnownAccounts(knownAccountSearchQuery(searchQuery));
  }, [api]);

  function openAccount(account: MastodonAccount) {
    globalThis.location.hash = accountHref(account.id).replace(/^\//, "");
    onClose();
  }

  async function resolveAccount(searchQuery: string) {
    if (!api || !searchQuery.trim()) return;
    setIsResolving(true);

    try {
      if (isLocalUrl(searchQuery)) {
        return [];
      }
      const search = await api.searchAccounts(normalizeRemoteQuery(searchQuery));
      return search.accounts;
    } finally {
      setIsResolving(false);
    }
  }

  return (
    <>
      <button
        type="button"
        className="fixed inset-0 top-16 z-20 cursor-default bg-background/45 backdrop-blur-sm"
        aria-label="Close search"
        onClick={onClose}
      />
      <div className="absolute left-0 right-0 top-full z-30 border-t border-border bg-transparent px-4 pt-3 md:px-6">
        <div className="mx-auto max-w-3xl">
          <div className="rounded-lg border border-border bg-card p-4 shadow-xl">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-semibold">Search</p>
                <p className="text-xs text-muted-foreground">Find people by handle or profile URL.</p>
              </div>
              <Button type="button" variant="ghost" size="icon" aria-label="Close search" onClick={onClose}>
                <X className="size-4" aria-hidden="true" />
              </Button>
            </div>
            <AccountCombobox
            value={query}
            onValueChange={(value) => {
              setQuery(value);
            }}
            searchKnownAccounts={searchKnownAccounts}
            isResolving={isResolving}
            placeholder="Search for people"
            onSelect={openAccount}
            onResolve={resolveAccount}
          />

          {query.trim().length === 0 ? (
            <div className="mt-4">
              <EmptyState title="Find people" description="Search a local handle, remote handle, or profile URL." />
            </div>
          ) : null}
          </div>
        </div>
      </div>
    </>
  );
}

function isLocalUrl(value: string) {
  try {
    const url = new URL(value);
    return url.hostname === globalThis.location.hostname;
  } catch {
    return false;
  }
}
