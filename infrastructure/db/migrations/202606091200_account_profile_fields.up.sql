ALTER TABLE accounts ADD COLUMN profile_fields TEXT NOT NULL DEFAULT '[]';
ALTER TABLE remote_accounts ADD COLUMN profile_fields TEXT NOT NULL DEFAULT '[]';
