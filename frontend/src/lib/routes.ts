export function accountHref(accountId: string) {
  return `/#/accounts/${encodeURIComponent(accountId)}`;
}

export function statusHref(statusId: string) {
  return `/#/statuses/${encodeURIComponent(statusId)}`;
}

export function decodeRouteParam(value: string | undefined) {
  return value ? decodeURIComponent(value) : "";
}
