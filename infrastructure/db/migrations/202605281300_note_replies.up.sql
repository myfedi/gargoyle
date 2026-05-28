ALTER TABLE notes ADD COLUMN in_reply_to_id CHAR(26);

--bun:split

ALTER TABLE notes ADD COLUMN in_reply_to_uri TEXT;

--bun:split

CREATE INDEX notes_in_reply_to_id_idx ON notes(local_account_id, in_reply_to_id);
