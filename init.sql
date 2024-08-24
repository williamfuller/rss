DROP DATABASE IF EXISTS rss;

CREATE DATABASE rss;
\connect rss;

CREATE TABLE feeds (id serial primary key NOT NULL, name text NOT NULL, url text NOT NULL);
