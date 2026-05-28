export type TokenResponse = {
  access_token: string;
  token_type: string;
  scope?: string;
  created_at?: number;
  expires_in?: number;
  refresh_token?: string;
};

export type AuthSession = {
  accessToken: string;
  tokenType: string;
  scope?: string;
  createdAt?: number;
  expiresIn?: number;
  refreshToken?: string;
};
