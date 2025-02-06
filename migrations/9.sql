UPDATE feed_entries SET content = '' WHERE content IS NULL;
ALTER TABLE feed_entries ALTER content SET NOT NULL;
