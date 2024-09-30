ALTER TABLE feed_entries ADD CONSTRAINT feed_id_link_key UNIQUE (feed_id, link);

