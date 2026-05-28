type StatusContentProps = {
  html: string;
};

type ContentPart =
  | { type: "text"; value: string }
  | { type: "link"; value: string; href: string };

export function StatusContent({ html }: StatusContentProps) {
  const parts = parseStatusContent(html);

  return (
    <p className="whitespace-pre-wrap text-sm leading-6">
      {parts.map((part, index) => {
        if (part.type === "link") {
          return (
            <a key={`${part.href}-${index}`} className="text-primary hover:underline" href={part.href} target="_blank" rel="noreferrer">
              {part.value}
            </a>
          );
        }
        return <span key={`${part.value}-${index}`}>{part.value}</span>;
      })}
    </p>
  );
}

function parseStatusContent(html: string): ContentPart[] {
  const document = new DOMParser().parseFromString(html, "text/html");
  const parts: ContentPart[] = [];

  function walk(node: Node) {
    if (node.nodeType === Node.TEXT_NODE) {
      pushText(parts, node.textContent ?? "");
      return;
    }

    if (node instanceof HTMLAnchorElement) {
      const text = node.textContent ?? "";
      const href = node.href;
      if (text && isSafeHttpUrl(href)) {
        parts.push({ type: "link", value: text, href });
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
  return linkifyMentions(parts).filter((part) => part.value.length > 0);
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

function linkifyMentions(parts: ContentPart[]) {
  return parts.flatMap((part): ContentPart[] => {
    if (part.type !== "text") {
      return [part];
    }

    const result: ContentPart[] = [];
    const mentionPattern = /(^|\s)(@[a-zA-Z0-9_][a-zA-Z0-9_.-]*@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = mentionPattern.exec(part.value))) {
      const prefix = match[1] ?? "";
      const mention = match[2] ?? "";
      const mentionStart = match.index + prefix.length;

      pushText(result, part.value.slice(lastIndex, mentionStart));
      const [, handle, host] = mention.match(/^@(.+)@([^@]+)$/) ?? [];
      if (handle && host) {
        result.push({ type: "link", value: mention, href: `https://${host}/@${handle}` });
      } else {
        pushText(result, mention);
      }
      lastIndex = mentionStart + mention.length;
    }

    pushText(result, part.value.slice(lastIndex));
    return result;
  });
}

function isSafeHttpUrl(value: string) {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}
