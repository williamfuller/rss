package main

import "database/sql"

func newDB() (*sql.DB, error) {
	const DatabaseURL = "postgres://postgres@:5432/rss?sslmode=disable"
	return sql.Open("postgres", DatabaseURL)
}
