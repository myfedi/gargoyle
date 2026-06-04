DROP INDEX IF EXISTS poll_votes_note_local_idx;
DROP INDEX IF EXISTS poll_votes_remote_option_idx;
DROP INDEX IF EXISTS poll_votes_local_option_idx;
DROP TABLE IF EXISTS poll_votes;
DROP INDEX IF EXISTS poll_options_note_position_idx;
DROP TABLE IF EXISTS poll_options;
ALTER TABLE notes DROP COLUMN poll_expires_at;
ALTER TABLE notes DROP COLUMN poll_multiple;
