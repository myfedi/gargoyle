export type MastodonAccount = {
  id: string;
  username: string;
  acct: string;
  display_name: string;
  url: string;
  avatar?: string;
  avatar_static?: string;
  bot?: boolean;
  note?: string;
  followers_count?: number;
  following_count?: number;
  statuses_count?: number;
};

export type MastodonStatus = {
  id: string;
  uri: string;
  url: string;
  created_at: string;
  account: MastodonAccount;
  content: string;
  visibility: "public" | "unlisted" | "private" | "direct" | string;
  sensitive: boolean;
  spoiler_text: string;
  replies_count: number;
  reblogs_count: number;
  favourites_count: number;
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
