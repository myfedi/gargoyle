export type MastodonAccountField = {
  name: string;
  value: string;
  verified_at?: string | null;
};

export type MastodonAccount = {
  id: string;
  username: string;
  acct: string;
  display_name: string;
  url: string;
  avatar?: string;
  avatar_static?: string;
  header?: string;
  header_static?: string;
  bot?: boolean;
  locked?: boolean;
  note?: string;
  followers_count?: number;
  following_count?: number;
  statuses_count?: number;
  fields?: MastodonAccountField[];
};

export type DomainBlock = {
  id: string;
  domain: string;
  severity: string;
  reject_media: boolean;
  public_comment?: string | null;
  private_comment?: string | null;
  created_by_user_id: string;
  created_at: string;
  updated_at: string;
};

export type ModerationJob = {
  id: string;
  kind: string;
  status: string;
};

export type MastodonRelationship = {
  id: string;
  following: boolean;
  showing_reblogs: boolean;
  notifying: boolean;
  followed_by: boolean;
  blocking: boolean;
  blocked_by: boolean;
  muting: boolean;
  muting_notifications: boolean;
  requested: boolean;
  domain_blocking: boolean;
  endorsed: boolean;
};

export type MastodonSearchResults = {
  accounts: MastodonAccount[];
  statuses: MastodonStatus[];
  hashtags: unknown[];
};

export type MastodonMediaAttachment = {
  id: string;
  type: "image" | "video" | "gifv" | "audio" | "unknown" | string;
  url: string;
  preview_url: string;
  description?: string;
};

export type MastodonMention = {
  id: string;
  username: string;
  acct: string;
  url: string;
};

export type ActivityPubObjectType = "Note" | "Article" | "Page" | "Question";

export type MastodonPoll = {
  id: string;
  expires_at?: string | null;
  expired: boolean;
  multiple: boolean;
  votes_count: number;
  voters_count: number;
  voted: boolean;
  own_votes: number[];
  options: Array<{ title: string; votes_count: number }>;
};

export type MastodonTag = {
  name: string;
  url: string;
};

export type MastodonCustomEmoji = {
  shortcode: string;
  url: string;
  static_url: string;
  visible_in_picker?: boolean;
};

export type MastodonStatus = {
  id: string;
  uri: string;
  url: string;
  created_at: string;
  account: MastodonAccount;
  content: string;
  visibility: "public" | "unlisted" | "private" | "direct" | string;
  activitypub_type?: ActivityPubObjectType | string;
  sensitive: boolean;
  spoiler_text: string;
  replies_count: number;
  reblogs_count: number;
  favourites_count: number;
  favourited?: boolean;
  reblogged?: boolean;
  bookmarked?: boolean;
  pinned?: boolean;
  media_attachments?: MastodonMediaAttachment[];
  mentions?: MastodonMention[];
  in_reply_to_id?: string | null;
  in_reply_to_account_id?: string | null;
  reblog?: MastodonStatus | null;
  poll?: MastodonPoll | null;
  tags?: MastodonTag[];
  emojis?: MastodonCustomEmoji[];
};

export type MastodonNotification = {
  id: string;
  type: string;
  created_at: string;
  account: MastodonAccount;
  status?: MastodonStatus;
};

export type MastodonConversation = {
  id: string;
  unread: boolean;
  accounts: MastodonAccount[];
  last_status?: MastodonStatus | null;
};

export type MastodonInstance = {
  uri?: string;
  domain?: string;
  title: string;
  short_description?: string;
  description?: string;
  version: string;
  stats?: {
    user_count: number;
    status_count: number;
    domain_count: number;
  };
};
