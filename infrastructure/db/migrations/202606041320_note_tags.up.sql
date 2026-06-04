ALTER TABLE notes ADD COLUMN hashtags_json TEXT;

--bun:split

ALTER TABLE notes ADD COLUMN emojis_json TEXT;
