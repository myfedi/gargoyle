import { accountHref } from "@/lib/routes";
import type { MastodonCustomEmoji, MastodonMention, MastodonTag } from "@/types/mastodon";

type StatusContentProps = {
  html: string;
  mentions?: MastodonMention[];
  tags?: MastodonTag[];
  emojis?: MastodonCustomEmoji[];
};

type ContentPart =
  | { type: "text"; value: string }
  | { type: "link"; value: string; href: string; internal: boolean }
  | { type: "emoji"; shortcode: string; url: string };

export function StatusContent({ html, mentions = [], tags = [], emojis = [] }: StatusContentProps) {
  const parts = parseStatusContent(html, mentions, tags, emojis);

  return (
    <p className="whitespace-pre-wrap text-sm leading-6">
      {parts.map((part, index) => {
        if (part.type === "link") {
          return (
            <a
              key={`${part.href}-${index}`}
              className="text-primary hover:underline"
              href={part.href}
              target={part.internal ? undefined : "_blank"}
              rel={part.internal ? undefined : "noreferrer"}
            >
              {part.value}
            </a>
          );
        }
        if (part.type === "emoji") {
          return <img key={`${part.shortcode}-${index}`} className="mx-0.5 inline-block size-5 align-[-0.2em]" src={part.url} alt={`:${part.shortcode}:`} title={`:${part.shortcode}:`} />;
        }
        return <span key={`${part.value}-${index}`}>{part.value}</span>;
      })}
    </p>
  );
}

function parseStatusContent(html: string, mentions: MastodonMention[], tags: MastodonTag[], emojis: MastodonCustomEmoji[]): ContentPart[] {
  const document = new DOMParser().parseFromString(html, "text/html");
  const parts: ContentPart[] = [];
  const mentionLookup = createMentionLookup(mentions);

  function walk(node: Node) {
    if (node.nodeType === Node.TEXT_NODE) {
      pushText(parts, node.textContent ?? "");
      return;
    }

    if (node instanceof HTMLAnchorElement) {
      const text = node.textContent ?? "";
      const href = node.href;
      const mention = findMentionForLink(text, href, mentionLookup);
      if (mention) {
        parts.push({ type: "link", value: text || `@${mention.acct}`, href: accountHref(mention.id), internal: true });
      } else if (text && isSafeHttpUrl(href)) {
        parts.push({ type: "link", value: text, href, internal: false });
      } else {
        pushText(parts, text);
      }
      return;
    }

    if (node instanceof HTMLBRElement) {
      pushText(parts, "\n");
      return;
    }

    node.childNodes.forEach(walk);

    if (node instanceof HTMLParagraphElement || node instanceof HTMLDivElement) {
      pushText(parts, "\n");
    }
  }

  document.body.childNodes.forEach(walk);
  return applyCustomEmojis(linkifyTags(linkifyMentions(parts, mentionLookup), tags), emojis).filter((part) => part.type !== "text" || part.value.length > 0);
}

type MentionLookup = {
  byAcct: Map<string, MastodonMention>;
  byUrl: Map<string, MastodonMention>;
};

function createMentionLookup(mentions: MastodonMention[]): MentionLookup {
  const byAcct = new Map<string, MastodonMention>();
  const byUrl = new Map<string, MastodonMention>();

  for (const mention of mentions) {
    byAcct.set(normalizeMentionAcct(mention.acct), mention);
    byAcct.set(normalizeMentionAcct(mention.username), mention);
    byAcct.set(normalizeMentionAcct(`@${mention.acct}`), mention);
    byAcct.set(normalizeMentionAcct(`@${mention.username}`), mention);
    if (mention.url) {
      byUrl.set(normalizeUrl(mention.url), mention);
      const host = urlHost(mention.url);
      if (host) {
        byAcct.set(normalizeMentionAcct(`${mention.username}@${host}`), mention);
        byAcct.set(normalizeMentionAcct(`${mention.acct}@${host}`), mention);
        byAcct.set(normalizeMentionAcct(`@${mention.username}@${host}`), mention);
        byAcct.set(normalizeMentionAcct(`@${mention.acct}@${host}`), mention);
      }
    }
  }

  return { byAcct, byUrl };
}

function findMentionForLink(text: string, href: string, lookup: MentionLookup) {
  const byUrl = lookup.byUrl.get(normalizeUrl(href));
  if (byUrl) return byUrl;

  return lookup.byAcct.get(normalizeMentionAcct(text));
}

function pushText(parts: ContentPart[], value: string) {
  if (!value) {
    return;
  }
  const previous = parts.at(-1);
  if (previous?.type === "text") {
    previous.value += value;
    return;
  }
  parts.push({ type: "text", value });
}

function linkifyMentions(parts: ContentPart[], lookup: MentionLookup) {
  return parts.flatMap((part): ContentPart[] => {
    if (part.type !== "text") {
      return [part];
    }

    const result: ContentPart[] = [];
    const mentionPattern = /(^|\s)(@[a-zA-Z0-9_][a-zA-Z0-9_.-]*(?:@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})?)/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = mentionPattern.exec(part.value))) {
      const prefix = match[1] ?? "";
      const mentionText = match[2] ?? "";
      const mentionStart = match.index + prefix.length;

      pushText(result, part.value.slice(lastIndex, mentionStart));
      const mention = lookup.byAcct.get(normalizeMentionAcct(mentionText));
      if (mention) {
        result.push({ type: "link", value: mentionText, href: accountHref(mention.id), internal: true });
      } else {
        const [, handle, host] = mentionText.match(/^@(.+)@([^@]+)$/) ?? [];
        if (handle && host) {
          result.push({ type: "link", value: mentionText, href: `https://${host}/@${handle}`, internal: false });
        } else {
          pushText(result, mentionText);
        }
      }
      lastIndex = mentionStart + mentionText.length;
    }

    pushText(result, part.value.slice(lastIndex));
    return result;
  });
}

function linkifyTags(parts: ContentPart[], tags: MastodonTag[]) {
  const tagLookup = new Map(tags.map((tag) => [tag.name.toLowerCase(), tag]));
  return parts.flatMap((part): ContentPart[] => {
    if (part.type !== "text") return [part];
    const result: ContentPart[] = [];
    const pattern = /(^|\s)#([\p{L}\p{N}_]{1,64})/gu;
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = pattern.exec(part.value))) {
      const prefix = match[1] ?? "";
      const name = match[2] ?? "";
      const start = match.index + prefix.length;
      pushText(result, part.value.slice(lastIndex, start));
      const tag = tagLookup.get(name.toLowerCase());
      if (tag) {
        result.push({ type: "link", value: `#${name}`, href: tag.url || `/tags/${name}`, internal: true });
      } else {
        pushText(result, `#${name}`);
      }
      lastIndex = start + name.length + 1;
    }
    pushText(result, part.value.slice(lastIndex));
    return result;
  });
}

function applyCustomEmojis(parts: ContentPart[], emojis: MastodonCustomEmoji[]) {
  const lookup = new Map(emojis.map((emoji) => [emoji.shortcode, emoji]));
  return parts.flatMap((part): ContentPart[] => {
    if (part.type !== "text") return [part];
    const result: ContentPart[] = [];
    const pattern = /:([A-Za-z0-9_+-]{2,64}):/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;
    while ((match = pattern.exec(part.value))) {
      const shortcode = match[1] ?? "";
      pushText(result, part.value.slice(lastIndex, match.index));
      const emoji = lookup.get(shortcode);
      if (emoji) {
        result.push({ type: "emoji", shortcode, url: emoji.static_url || emoji.url });
      } else {
        pushText(result, match[0] ?? "");
      }
      lastIndex = match.index + (match[0]?.length ?? 0);
    }
    pushText(result, part.value.slice(lastIndex));
    return result;
  });
}

function normalizeMentionAcct(value: string) {
  return value.trim().replace(/^@/, "").toLowerCase();
}

function urlHost(value: string) {
  try {
    return new URL(value).host.toLowerCase();
  } catch {
    return null;
  }
}

function normalizeUrl(value: string) {
  try {
    const url = new URL(value);
    url.hash = "";
    url.search = "";
    return url.toString().replace(/\/$/, "").toLowerCase();
  } catch {
    return value.trim().replace(/\/$/, "").toLowerCase();
  }
}

function isSafeHttpUrl(value: string) {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}
