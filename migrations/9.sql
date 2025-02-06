UPDATE feed_entries SET content = '' WHERE content IS NULL;
ALTER TABLE feed_entries ADD COLUMN content text NOT NULL DEFAULT '';
