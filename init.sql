DROP DATABASE IF EXISTS rss;

CREATE DATABASE rss;
\connect rss;

CREATE TABLE migrations (name text NOT NULL);
CREATE UNIQUE INDEX migrations_name ON migrations(name);
