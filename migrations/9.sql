ALTER TABLE feed_entries DROP COLUMN content;
ALTER TABLE feed_entries ADD COLUMN content text NOT NULL;
