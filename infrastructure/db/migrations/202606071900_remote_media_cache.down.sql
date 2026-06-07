DROP INDEX IF EXISTS idx_media_attachments_remote_cache;
DROP INDEX IF EXISTS idx_media_attachments_remote_url;

ALTER TABLE media_attachments DROP COLUMN file_size;
ALTER TABLE media_attachments DROP COLUMN remote_last_accessed_at;
ALTER TABLE media_attachments DROP COLUMN remote_fetched_at;
ALTER TABLE media_attachments DROP COLUMN remote_url;
