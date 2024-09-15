CREATE TABLE feed_entries (
	id serial primary key NOT NULL,
	feed_id int NOT NULL REFERENCES feeds(id) ON DELETE CASCADE,
  title text NOT NULL,
	link text NOT NULL,
	description text NOT NULL
);
