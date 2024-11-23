package main

import (
	"database/sql"
	"net/http"
	"rss-app/rss"
)

type FeedEntry struct {
	Id     int
	FeedId int
	rss.Item
}

type FeedEntriesController struct{}

func (f *FeedEntriesController) Show(d *sql.DB, w http.ResponseWriter, r *http.Request) (*Response, string, error) {
	feedEntry := FeedEntry{Id: idPathValue(r)}
	err := d.QueryRowContext(r.Context(), `
	SELECT title, link, description, pub_date 
	FROM feed_entries
	WHERE id = $1
	ORDER by pub_date DESC, title`, feedEntry.Id).Scan(&feedEntry.Title, &feedEntry.Link, &feedEntry.Description, &feedEntry.PubDate.Time)
	if err != nil {
		return nil, "", err
	}

	return &Response{Data: feedEntry}, "html/feed_entries/show.html", nil
}
