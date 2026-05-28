import { accountHref } from "@/lib/routes";
import type { MastodonMention } from "@/types/mastodon";

type StatusContentProps = {
  html: string;
  mentions?: MastodonMention[];
};

type ContentPart =
  | { type: "text"; value: string }
  | { type: "link"; value: string; href: string; internal: boolean };

export function StatusContent({ html, mentions = [] }: StatusContentProps) {
  const parts = parseStatusContent(html, mentions);

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
        return <span key={`${part.value}-${index}`}>{part.value}</span>;
      })}
    </p>
  );
}

function parseStatusContent(html: string, mentions: MastodonMention[]): ContentPart[] {
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
  return linkifyMentions(parts, mentionLookup).filter((part) => part.value.length > 0);
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

function normalizeMentionAcct(value: string) {
  return value.trim().replace(/^@/, "").toLowerCase();
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
