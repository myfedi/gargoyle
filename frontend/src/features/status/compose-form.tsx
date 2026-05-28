import type React from "react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { MastodonAccount, MastodonMediaAttachment } from "@/types/mastodon";

export type ComposeValues = {
  status: string;
  visibility: "public" | "unlisted" | "private" | "direct";
  sensitive: boolean;
  spoilerText: string;
  mediaIds: string[];
};

type ComposeFormProps = {
  submitLabel: string;
  submittingLabel: string;
  placeholder: string;
  isSubmitting: boolean;
  error?: string | null;
  initialText?: string;
  onSubmit: (values: ComposeValues) => Promise<void> | void;
  onUploadMedia?: (file: File, description?: string) => Promise<MastodonMediaAttachment>;
  onDeleteMedia?: (id: string) => Promise<void>;
  onUpdateMedia?: (id: string, description: string) => Promise<MastodonMediaAttachment>;
  searchKnownAccounts?: (query: string) => Promise<MastodonAccount[]>;
};

const maxLength = 500;

export function ComposeForm({
  submitLabel,
  submittingLabel,
  placeholder,
  isSubmitting,
  error,
  initialText = "",
  onSubmit,
  onUploadMedia,
  onDeleteMedia,
  onUpdateMedia,
  searchKnownAccounts,
}: ComposeFormProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const [status, setStatus] = useState(initialText);
  const [visibility, setVisibility] = useState<ComposeValues["visibility"]>("public");
  const [sensitive, setSensitive] = useState(false);
  const [spoilerText, setSpoilerText] = useState("");
  const [media, setMedia] = useState<MastodonMediaAttachment | null>(null);
  const [mediaDescription, setMediaDescription] = useState("");
  const [mediaError, setMediaError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [isDeletingMedia, setIsDeletingMedia] = useState(false);
  const [isUpdatingMedia, setIsUpdatingMedia] = useState(false);
  const [caretPosition, setCaretPosition] = useState(0);
  const [mentionResults, setMentionResults] = useState<MastodonAccount[]>([]);
  const [isSearchingMentions, setIsSearchingMentions] = useState(false);
  const [mentionError, setMentionError] = useState<string | null>(null);
  const mentionQuery = currentMentionQuery(status, caretPosition);
  const mentionSearchQuery = mentionQuery?.endsWith("@") ? mentionQuery.slice(0, -1) : mentionQuery;
  const remaining = maxLength - status.length;

  useEffect(() => {
    if (!searchKnownAccounts || !mentionSearchQuery || mentionSearchQuery.length < 2) {
      setMentionResults([]);
      setMentionError(null);
      return;
    }

    let cancelled = false;
    const timeout = window.setTimeout(() => {
      setIsSearchingMentions(true);
      setMentionError(null);
      searchKnownAccounts(mentionSearchQuery)
        .then((accounts) => {
          if (!cancelled) setMentionResults(accounts);
        })
        .catch((caughtError: unknown) => {
          if (!cancelled) setMentionError(caughtError instanceof Error ? caughtError.message : "Could not search accounts.");
        })
        .finally(() => {
          if (!cancelled) setIsSearchingMentions(false);
        });
    }, 180);

    return () => {
      cancelled = true;
      window.clearTimeout(timeout);
    };
  }, [mentionSearchQuery, searchKnownAccounts]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!status.trim() || remaining < 0) {
      return;
    }

    await onSubmit({ status: status.trim(), visibility, sensitive, spoilerText, mediaIds: media ? [media.id] : [] });
    setStatus("");
    setSensitive(false);
    setSpoilerText("");
    setMedia(null);
    setMediaDescription("");
  }

  function insertMention(account: MastodonAccount) {
    const token = mentionToken(status, caretPosition);
    if (!token) return;

    const mention = `@${account.acct} `;
    const nextStatus = `${status.slice(0, token.start)}${mention}${status.slice(token.end)}`;
    const nextCaretPosition = token.start + mention.length;
    setStatus(nextStatus);
    setCaretPosition(nextCaretPosition);
    setMentionResults([]);
    window.setTimeout(() => {
      textareaRef.current?.focus();
      textareaRef.current?.setSelectionRange(nextCaretPosition, nextCaretPosition);
    }, 0);
  }

  async function saveMediaDescription() {
    if (!media || !onUpdateMedia) {
      return;
    }

    setIsUpdatingMedia(true);
    setMediaError(null);

    try {
      const updated = await onUpdateMedia(media.id, mediaDescription.trim());
      setMedia(updated);
      setMediaDescription(updated.description ?? "");
    } catch (caughtError) {
      setMediaError(caughtError instanceof Error ? caughtError.message : "Could not update media description.");
    } finally {
      setIsUpdatingMedia(false);
    }
  }

  async function removeMedia() {
    if (!media) {
      return;
    }

    if (!onDeleteMedia) {
      setMedia(null);
      setMediaDescription("");
      return;
    }

    setIsDeletingMedia(true);
    setMediaError(null);

    try {
      await onDeleteMedia(media.id);
      setMedia(null);
      setMediaDescription("");
    } catch (caughtError) {
      setMediaError(caughtError instanceof Error ? caughtError.message : "Could not delete media.");
    } finally {
      setIsDeletingMedia(false);
    }
  }

  async function uploadSelectedFile(file: File | undefined) {
    if (!file || !onUploadMedia) {
      return;
    }

    setIsUploading(true);
    setMediaError(null);

    try {
      const attachment = await onUploadMedia(file, mediaDescription.trim() || undefined);
      setMedia(attachment);
    } catch (caughtError) {
      setMediaError(caughtError instanceof Error ? caughtError.message : "Could not upload media.");
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  }

  return (
    <form className="space-y-4" onSubmit={(event) => void submit(event)}>
      <div className="relative">
        <Textarea
          ref={textareaRef}
          value={status}
          onChange={(event) => {
            setStatus(event.target.value);
            setCaretPosition(event.target.selectionStart);
          }}
          onClick={(event) => setCaretPosition(event.currentTarget.selectionStart)}
          onKeyUp={(event) => setCaretPosition(event.currentTarget.selectionStart)}
          placeholder={placeholder}
          aria-label="Post content"
          rows={6}
        />
        {mentionSearchQuery && mentionSearchQuery.length >= 2 ? (
          <div className="absolute z-20 mt-2 w-full overflow-hidden rounded-lg border border-border bg-card shadow-lg">
            {isSearchingMentions ? (
              <p className="p-3 text-sm text-muted-foreground">Searching accounts...</p>
            ) : mentionError ? (
              <p className="p-3 text-sm text-destructive">{mentionError}</p>
            ) : mentionResults.length > 0 ? (
              <div className="max-h-64 overflow-auto p-1">
                {mentionResults.map((account) => (
                  <button
                    key={account.id}
                    type="button"
                    className="block w-full rounded-md px-3 py-2 text-left hover:bg-accent hover:text-accent-foreground"
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => insertMention(account)}
                  >
                    <span className="block truncate text-sm font-medium">{account.display_name || account.username}</span>
                    <span className="block truncate text-xs text-muted-foreground">@{account.acct}</span>
                  </button>
                ))}
              </div>
            ) : (
              <p className="p-3 text-sm text-muted-foreground">No known accounts.</p>
            )}
          </div>
        ) : null}
      </div>

      <div className="grid gap-3 md:grid-cols-[12rem_1fr]">
        <label className="space-y-1 text-sm font-medium">
          <span>Visibility</span>
          <select
            className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
            value={visibility}
            onChange={(event) => setVisibility(event.target.value as ComposeValues["visibility"])}
          >
            <option value="public">Public</option>
            <option value="unlisted">Unlisted</option>
            <option value="private">Private</option>
          </select>
        </label>

        <label className="space-y-1 text-sm font-medium">
          <span>Content warning</span>
          <Input value={spoilerText} onChange={(event) => setSpoilerText(event.target.value)} placeholder="Optional" />
        </label>
      </div>

      <label className="flex items-center gap-2 text-sm text-muted-foreground">
        <input type="checkbox" checked={sensitive} onChange={(event) => setSensitive(event.target.checked)} />
        Mark as sensitive
      </label>

      {onUploadMedia ? (
        <div className="rounded-md border border-border bg-background p-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
            <label className="flex-1 space-y-1 text-sm font-medium">
              <span>Media description</span>
              <Input value={mediaDescription} onChange={(event) => setMediaDescription(event.target.value)} placeholder="Optional alt text" />
            </label>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              className="hidden"
              onChange={(event) => void uploadSelectedFile(event.target.files?.[0])}
            />
            <Button type="button" variant="outline" onClick={() => fileInputRef.current?.click()} disabled={isUploading}>
              {isUploading ? "Uploading..." : "Upload media"}
            </Button>
          </div>
          {media ? (
            <div className="mt-3 space-y-3 rounded-md border border-border bg-card px-3 py-3">
              {media.type === "image" ? (
                <img className="max-h-48 rounded-md border border-border object-contain" src={media.preview_url || media.url} alt={media.description || "Uploaded media preview"} />
              ) : null}
              <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
                <label className="flex-1 space-y-1 text-sm font-medium">
                  <span>Alt text</span>
                  <Input value={mediaDescription} onChange={(event) => setMediaDescription(event.target.value)} placeholder="Describe the media" />
                </label>
                <Button type="button" variant="outline" onClick={() => void saveMediaDescription()} disabled={isUpdatingMedia}>
                  {isUpdatingMedia ? "Saving..." : "Save alt text"}
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  disabled={isDeletingMedia}
                  onClick={() => void removeMedia()}
                >
                  {isDeletingMedia ? "Removing..." : "Remove"}
                </Button>
              </div>
            </div>
          ) : null}
          {mediaError ? <p className="mt-2 text-sm text-destructive">{mediaError}</p> : null}
        </div>
      ) : null}

      {error ? (
        <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className={remaining < 0 ? "text-sm text-destructive" : "text-sm text-muted-foreground"}>
          {remaining} characters remaining
        </p>
        <Button type="submit" disabled={isSubmitting || !status.trim() || remaining < 0}>
          {isSubmitting ? submittingLabel : submitLabel}
        </Button>
      </div>
    </form>
  );
}

function currentMentionQuery(text: string, caretPosition: number) {
  const token = mentionToken(text, caretPosition);
  return token?.query ?? null;
}

function mentionToken(text: string, caretPosition: number) {
  const beforeCaret = text.slice(0, caretPosition);
  const match = beforeCaret.match(/(^|\s)@(\S*)$/);
  if (!match || match.index === undefined) return null;
  const prefixLength = match[1].length;
  const start = match.index + prefixLength;
  return { start, end: caretPosition, query: match[2] };
}
