import { useCallback, useMemo, useState } from "react";

import { useAuth } from "@/app/auth-context";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { AccountCombobox, normalizeRemoteQuery } from "@/features/accounts/account-combobox";
import { createMastodonApi } from "@/lib/mastodon-api";
import type { MastodonAccount, MastodonStatus } from "@/types/mastodon";

type DirectMessageFormProps = {
  forwardedStatus?: MastodonStatus;
  onSent?: () => void;
  onCancel?: () => void;
};

const maxLength = 500;

export function DirectMessageForm({ forwardedStatus, onSent, onCancel }: DirectMessageFormProps) {
  const { session } = useAuth();
  const [query, setQuery] = useState("");
  const [recipient, setRecipient] = useState<MastodonAccount | null>(null);
  const [text, setText] = useState("");
  const [isResolving, setIsResolving] = useState(false);
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const api = useMemo(() => (session?.accessToken ? createMastodonApi(session.accessToken) : null), [session?.accessToken]);
  const remaining = maxLength - directBody(text, forwardedStatus).length;

  const searchKnownAccounts = useCallback(async (searchQuery: string) => {
    if (!api) return [];
    return api.searchKnownAccounts(searchQuery);
  }, [api]);

  async function resolveAccount(searchQuery: string) {
    if (!api || !searchQuery.trim()) return;
    setIsResolving(true);
    setError(null);

    try {
      const search = await api.searchAccounts(normalizeRemoteQuery(searchQuery));
      return search.accounts;
    } finally {
      setIsResolving(false);
    }
  }

  function chooseRecipient(account: MastodonAccount) {
    setRecipient(account);
    setQuery(account.acct);
  }

  async function sendDirectMessage() {
    if (!api || !recipient || remaining < 0) return;
    const body = directBody(text, forwardedStatus).trim();
    if (!body) return;

    setIsSending(true);
    setError(null);

    try {
      await api.createStatus({
        status: withMention(body, recipient),
        visibility: "direct",
      });
      setText("");
      setRecipient(null);
      setQuery("");
      onSent?.();
    } catch (caughtError) {
      setError(caughtError instanceof Error ? caughtError.message : "Could not send direct message.");
    } finally {
      setIsSending(false);
    }
  }

  return (
    <div className="space-y-4">
      <AccountCombobox
        value={query}
        onValueChange={(value) => {
          setQuery(value);
          setRecipient(null);
        }}
        searchKnownAccounts={searchKnownAccounts}
        isResolving={isResolving}
        placeholder="Choose a recipient"
        onSelect={chooseRecipient}
        onResolve={resolveAccount}
      />

      <Textarea
        value={text}
        onChange={(event) => setText(event.target.value)}
        placeholder={forwardedStatus ? "Add a note" : "Write a direct message"}
        aria-label={forwardedStatus ? "Forward note" : "Direct message"}
        rows={4}
      />

      {forwardedStatus ? (
        <div className="rounded-md border border-border bg-background p-3 text-sm text-muted-foreground">
          Forwarding post by @{forwardedStatus.account.acct}
        </div>
      ) : null}

      {error ? <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">{error}</p> : null}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className={remaining < 0 ? "text-sm text-destructive" : "text-sm text-muted-foreground"}>{remaining} characters remaining</p>
        <div className="flex justify-end gap-2">
          {onCancel ? <Button type="button" variant="ghost" onClick={onCancel}>Cancel</Button> : null}
          <Button type="button" disabled={isSending || !recipient || remaining < 0 || !directBody(text, forwardedStatus).trim()} onClick={() => void sendDirectMessage()}>
            {isSending ? "Sending..." : forwardedStatus ? "Forward" : "Send"}
          </Button>
        </div>
      </div>
    </div>
  );
}

function directBody(text: string, forwardedStatus?: MastodonStatus) {
  const trimmed = text.trim();
  if (!forwardedStatus) return trimmed;
  const statusUrl = forwardedStatus.url || forwardedStatus.uri;
  return [trimmed, statusUrl].filter(Boolean).join("\n\n");
}

function withMention(text: string, recipient: MastodonAccount) {
  const mention = `@${recipient.acct}`;
  return text.startsWith(mention) ? text : `${mention} ${text}`;
}
