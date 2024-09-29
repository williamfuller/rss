package main

import (
	"database/sql"
	"html/template"
	"net/http"
)

type FeedEntry struct {
	Id     int
	FeedId int
	Item
}

type FeedEntriessController struct{}

func (f *FeedEntriessController) Show(d *sql.DB, w http.ResponseWriter, r *http.Request) {
	feedEntry := FeedEntry{Id: idPathValue(r)}
	err := d.QueryRowContext(r.Context(), `
	SELECT title, link, description, pub_date 
	FROM feed_entries
	WHERE id = $1
	ORDER by pub_date DESC, title`, feedEntry.Id).Scan(&feedEntry.Title, &feedEntry.Link, &feedEntry.Description, &feedEntry.PubDate.Time)
	if err != nil {
		panic(err)
	}

	tmplt, err := template.ParseFiles("templates/feed_entry.html")
	if err != nil {
		panic(err)
	}

	err = tmplt.Execute(w, &feedEntry)
	if err != nil {
		panic(err)
	}
}
