import type React from "react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { ActivityPubObjectType, MastodonAccount, MastodonMediaAttachment } from "@/types/mastodon";

export type ComposeValues = {
  status: string;
  visibility: "public" | "unlisted" | "private" | "direct";
  sensitive: boolean;
  spoilerText: string;
  mediaIds: string[];
  objectType: ActivityPubObjectType;
  pollOptions: string[];
  pollMultiple: boolean;
  pollExpiresIn: number;
};

type ComposeFormProps = {
  submitLabel: string;
  submittingLabel: string;
  placeholder: string;
  isSubmitting: boolean;
  error?: string | null;
  initialText?: string;
  initialVisibility?: ComposeValues["visibility"];
  initialSensitive?: boolean;
  initialSpoilerText?: string;
  initialMedia?: MastodonMediaAttachment | null;
  initialObjectType?: ActivityPubObjectType;
  initialPollOptions?: string[];
  initialPollMultiple?: boolean;
  initialPollExpiresIn?: number;
  resetAfterSubmit?: boolean;
  onSubmit: (values: ComposeValues) => Promise<void> | void;
  onUploadMedia?: (file: File, description?: string) => Promise<MastodonMediaAttachment>;
  onDeleteMedia?: (id: string) => Promise<void>;
  onUpdateMedia?: (id: string, description: string) => Promise<MastodonMediaAttachment>;
  searchKnownAccounts?: (query: string) => Promise<MastodonAccount[]>;
  compact?: boolean;
};

const objectTypeOptions: Array<{ value: ActivityPubObjectType; label: string; hint: string; maxLength: number }> = [
  { value: "Note", label: "Post", hint: "Short fediverse status", maxLength: 500 },
  { value: "Article", label: "Article", hint: "Long-form writing", maxLength: 5000 },
  { value: "Page", label: "Page", hint: "Stable reference page", maxLength: 5000 },
  { value: "Question", label: "Question", hint: "Question-shaped post", maxLength: 1000 },
];

const defaultPollOptions = ["", ""];
const defaultPollExpiresIn = 24 * 60 * 60;

export function ComposeForm({
  submitLabel,
  submittingLabel,
  placeholder,
  isSubmitting,
  error,
  initialText = "",
  initialVisibility = "public",
  initialSensitive = false,
  initialSpoilerText = "",
  initialMedia = null,
  initialObjectType = "Note",
  initialPollOptions = defaultPollOptions,
  initialPollMultiple = false,
  initialPollExpiresIn = defaultPollExpiresIn,
  resetAfterSubmit = true,
  onSubmit,
  onUploadMedia,
  onDeleteMedia,
  onUpdateMedia,
  searchKnownAccounts,
  compact = false,
}: ComposeFormProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);
  const [status, setStatus] = useState(initialText);
  const [visibility, setVisibility] = useState<ComposeValues["visibility"]>(initialVisibility);
  const [sensitive, setSensitive] = useState(initialSensitive);
  const [spoilerText, setSpoilerText] = useState(initialSpoilerText);
  const [media, setMedia] = useState<MastodonMediaAttachment | null>(initialMedia);
  const [objectType, setObjectType] = useState<ActivityPubObjectType>(initialObjectType);
  const [pollOptions, setPollOptions] = useState(initialPollOptions.length >= 2 ? initialPollOptions : defaultPollOptions);
  const [pollMultiple, setPollMultiple] = useState(initialPollMultiple);
  const [pollExpiresIn, setPollExpiresIn] = useState(initialPollExpiresIn);
  const [mediaDescription, setMediaDescription] = useState(initialMedia?.description ?? "");
  const [mediaError, setMediaError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [isDeletingMedia, setIsDeletingMedia] = useState(false);
  const [isUpdatingMedia, setIsUpdatingMedia] = useState(false);
  const [caretPosition, setCaretPosition] = useState(0);
  const [mentionResults, setMentionResults] = useState<MastodonAccount[]>([]);
  const [isSearchingMentions, setIsSearchingMentions] = useState(false);
  const [mentionError, setMentionError] = useState<string | null>(null);
  const [isExpanded, setIsExpanded] = useState(!compact || initialText.length > 0);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const initialPollOptionsKey = initialPollOptions.join("\u0000");

  useEffect(() => {
    setStatus(initialText);
    setVisibility(initialVisibility);
    setSensitive(initialSensitive);
    setSpoilerText(initialSpoilerText);
    setMedia(initialMedia);
    setObjectType(initialObjectType);
    setPollOptions(initialPollOptions.length >= 2 ? initialPollOptions : defaultPollOptions);
    setPollMultiple(initialPollMultiple);
    setPollExpiresIn(initialPollExpiresIn);
    setMediaDescription(initialMedia?.description ?? "");
    setIsExpanded(!compact || initialText.length > 0);
  }, [compact, initialMedia, initialObjectType, initialPollExpiresIn, initialPollMultiple, initialPollOptionsKey, initialSensitive, initialSpoilerText, initialText, initialVisibility]);

  const mentionQuery = currentMentionQuery(status, caretPosition);
  const mentionSearchQuery = mentionQuery?.endsWith("@") ? mentionQuery.slice(0, -1) : mentionQuery;
  const maxLength = objectTypeOptions.find((option) => option.value === objectType)?.maxLength ?? 500;
  const remaining = maxLength - status.length;
  const validPollOptions = pollOptions.map((option) => option.trim()).filter(Boolean);
  const pollInvalid = objectType === "Question" && validPollOptions.length < 2;

  useEffect(() => {
    if (!searchKnownAccounts || !mentionSearchQuery || mentionSearchQuery.length < 2) {
      setMentionResults([]);
      setMentionError(null);
      return;
    }

    let cancelled = false;
    const timeout = globalThis.setTimeout(() => {
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
      globalThis.clearTimeout(timeout);
    };
  }, [mentionSearchQuery, searchKnownAccounts]);

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!status.trim() || remaining < 0 || pollInvalid) {
      return;
    }

    await onSubmit({ status: status.trim(), visibility, sensitive, spoilerText, mediaIds: media ? [media.id] : [], objectType, pollOptions: pollOptions.map((option) => option.trim()).filter(Boolean), pollMultiple, pollExpiresIn });
    if (resetAfterSubmit) {
      setStatus("");
      setSensitive(false);
      setSpoilerText("");
      setMedia(null);
      setMediaDescription("");
      setObjectType("Note");
      setPollOptions(defaultPollOptions);
      setPollMultiple(false);
      setPollExpiresIn(defaultPollExpiresIn);
      setShowAdvanced(false);
      if (compact) {
        setIsExpanded(false);
      }
    }
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
    globalThis.setTimeout(() => {
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
            if (compact) setIsExpanded(true);
          }}
          onClick={(event) => setCaretPosition(event.currentTarget.selectionStart)}
          onFocus={() => compact && setIsExpanded(true)}
          onKeyUp={(event) => setCaretPosition(event.currentTarget.selectionStart)}
          placeholder={placeholder}
          aria-label="Post content"
          rows={isExpanded ? 5 : 1}
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

      {isExpanded ? (
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-3">
            <Button type="button" variant="ghost" size="sm" onClick={() => setShowAdvanced((current) => !current)}>
              {showAdvanced ? "Hide options" : "Options"}
            </Button>
            {onUploadMedia ? (
              <>
                <input
                  ref={fileInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(event) => void uploadSelectedFile(event.target.files?.[0])}
                />
                <Button type="button" variant="ghost" size="sm" onClick={() => fileInputRef.current?.click()} disabled={isUploading}>
                  {isUploading ? "Uploading..." : media ? "Change media" : "Add media"}
                </Button>
              </>
            ) : null}
          </div>

          {showAdvanced ? (
            <div className="space-y-4 rounded-md border border-border bg-background p-3">
              <div className="grid gap-3 md:grid-cols-[12rem_12rem_1fr]">
                <label className="space-y-1 text-sm font-medium">
                  <span>Format</span>
                  <select
                    className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm"
                    value={objectType}
                    onChange={(event) => setObjectType(event.target.value as ActivityPubObjectType)}
                  >
                    {objectTypeOptions.map((option) => (
                      <option key={option.value} value={option.value}>{option.label}</option>
                    ))}
                  </select>
                  <span className="block text-xs font-normal text-muted-foreground">
                    {objectTypeOptions.find((option) => option.value === objectType)?.hint}
                  </span>
                </label>

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

              {objectType === "Question" ? (
                <div className="space-y-3 rounded-md border border-border bg-card p-3">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-sm font-medium">Poll options</p>
                      <p className="text-xs text-muted-foreground">Two to four choices.</p>
                    </div>
                    <Button type="button" variant="outline" size="sm" disabled={pollOptions.length >= 4} onClick={() => setPollOptions((current) => [...current, ""])}>
                      Add option
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {pollOptions.map((option, index) => (
                      <div key={index} className="flex gap-2">
                        <Input value={option} onChange={(event) => setPollOptions((current) => current.map((item, itemIndex) => itemIndex === index ? event.target.value : item))} placeholder={`Option ${index + 1}`} />
                        <Button type="button" variant="ghost" size="sm" disabled={pollOptions.length <= 2} onClick={() => setPollOptions((current) => current.filter((_, itemIndex) => itemIndex !== index))}>
                          Remove
                        </Button>
                      </div>
                    ))}
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <label className="space-y-1 text-sm font-medium">
                      <span>Poll duration</span>
                      <select className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-sm" value={pollExpiresIn} onChange={(event) => setPollExpiresIn(Number(event.target.value))}>
                        <option value={300}>5 minutes</option>
                        <option value={3600}>1 hour</option>
                        <option value={86400}>1 day</option>
                        <option value={604800}>1 week</option>
                      </select>
                    </label>
                    <label className="flex items-center gap-2 pt-6 text-sm text-muted-foreground">
                      <input type="checkbox" checked={pollMultiple} onChange={(event) => setPollMultiple(event.target.checked)} />
                      <span>Allow multiple choices</span>
                    </label>
                  </div>
                </div>
              ) : null}

              <label className="flex items-center gap-2 text-sm text-muted-foreground">
                <input type="checkbox" checked={sensitive} onChange={(event) => setSensitive(event.target.checked)} />
                <span>Mark as sensitive</span>
              </label>
            </div>
          ) : null}

          {media ? (
            <div className="space-y-3 rounded-md border border-border bg-background px-3 py-3">
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
                <Button type="button" variant="ghost" disabled={isDeletingMedia} onClick={() => void removeMedia()}>
                  {isDeletingMedia ? "Removing..." : "Remove"}
                </Button>
              </div>
            </div>
          ) : null}
          {mediaError ? <p className="text-sm text-destructive">{mediaError}</p> : null}
        </div>
      ) : null}

      {error ? (
        <p className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
          {error}
        </p>
      ) : null}

      {isExpanded ? (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <p className={remaining < 0 ? "text-sm text-destructive" : "text-sm text-muted-foreground"}>
            {remaining} characters remaining
          </p>
          <div className="flex justify-end gap-2">
            {compact ? (
              <Button type="button" variant="ghost" disabled={isSubmitting} onClick={() => { setIsExpanded(false); setShowAdvanced(false); }}>
                Collapse
              </Button>
            ) : null}
            <Button type="submit" disabled={isSubmitting || !status.trim() || remaining < 0 || pollInvalid}>
              {isSubmitting ? submittingLabel : submitLabel}
            </Button>
          </div>
        </div>
      ) : null}
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
