export function htmlToPlainText(html: string) {
  const document = new DOMParser().parseFromString(html, "text/html");
  return document.body.textContent?.trim() ?? "";
}

export function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }

  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}
