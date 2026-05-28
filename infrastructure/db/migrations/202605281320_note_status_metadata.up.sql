ALTER TABLE notes ADD COLUMN visibility TEXT NOT NULL DEFAULT 'public';

--bun:split

ALTER TABLE notes ADD COLUMN sensitive BOOLEAN NOT NULL DEFAULT FALSE;

--bun:split

ALTER TABLE notes ADD COLUMN spoiler_text TEXT;
