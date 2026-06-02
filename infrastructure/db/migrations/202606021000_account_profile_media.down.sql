ALTER TABLE accounts DROP COLUMN avatar_media_id;
--bun:split
ALTER TABLE accounts DROP COLUMN header_media_id;
--bun:split
ALTER TABLE accounts DROP COLUMN avatar_url;
--bun:split
ALTER TABLE accounts DROP COLUMN header_url;
--bun:split
ALTER TABLE remote_accounts DROP COLUMN avatar_url;
--bun:split
ALTER TABLE remote_accounts DROP COLUMN header_url;
