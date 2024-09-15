CREATE TABLE feeds (
	id serial primary key NOT NULL,
	title text NOT NULL,
	url text NOT NULL,
	link text NOT NULL,
	description text NOT NULL,
	updated_at timestamp with time zone NOT NULL DEFAULT NOW()
);
