CREATE TABLE feed_entries (
	id serial primary key NOT NULL,
	feed_id int NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  title text NOT NULL,
	link text NOT NULL,
	description text NOT NULL
);

ALTER TABLE feeds ADD COLUMN updated_at timestamp with time zone NOT NULL DEFAULT NOW();
ALTER TABLE feeds ADD COLUMN ttl int NOT NULL DEFAULT 0;
