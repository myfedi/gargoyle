ALTER TABLE accounts ADD COLUMN avatar_media_id CHAR(26);
--bun:split
ALTER TABLE accounts ADD COLUMN header_media_id CHAR(26);
--bun:split
ALTER TABLE accounts ADD COLUMN avatar_url TEXT;
--bun:split
ALTER TABLE accounts ADD COLUMN header_url TEXT;
--bun:split
ALTER TABLE remote_accounts ADD COLUMN avatar_url TEXT;
--bun:split
ALTER TABLE remote_accounts ADD COLUMN header_url TEXT;
