ALTER TABLE media_attachments ADD COLUMN remote_url TEXT;
ALTER TABLE media_attachments ADD COLUMN remote_fetched_at TIMESTAMPTZ;
ALTER TABLE media_attachments ADD COLUMN remote_last_accessed_at TIMESTAMPTZ;
ALTER TABLE media_attachments ADD COLUMN file_size INTEGER NOT NULL DEFAULT 0;

CREATE INDEX idx_media_attachments_remote_url ON media_attachments(remote_url);
CREATE INDEX idx_media_attachments_remote_cache ON media_attachments(remote_last_accessed_at, remote_fetched_at) WHERE remote_url IS NOT NULL;
