package main

import (
	"database/sql"
	"errors"
	"html/template"
	"net/http"
)

func showFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := d.
			QueryContext(r.Context(), `
			SELECT
				feeds.title,
				feed_entries.title,
				feed_entries.link,
				feed_entries.description
			FROM
				feeds,
				feed_entries
			WHERE
				feeds.id = feed_entries.feed_id AND
				feeds.id = $1`, r.PathValue("Id"))
		if err != nil {
			panic(err)
		}

		var feed Feed
		for {
			hasRow := rows.Next()
			if !hasRow {
				if rows.Err() != nil {
					panic(rows.Err())
				}
				break
			}

			var title string
			var item Item
			err := rows.Scan(&title, &item.Title, &item.Link, &item.Description)
			if err != nil {
				panic(err)
			}

			feed.Title = title
			feed.Items = append(feed.Items, item)
		}

		tmplt, err := template.ParseFiles("templates/feed.html")
		if err != nil {
			panic(err)
		}

		err = tmplt.Execute(w, feed)
		if err != nil {
			panic(err)
		}
	}
}

func getEditFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var feed Feed
		err := d.
			QueryRowContext(r.Context(), "SELECT id, url FROM feeds WHERE id = $1", idPathValue(r)).
			Scan(&feed.Id, &feed.URL)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			panic(err)
		}

		tmplt, err := template.ParseFiles("templates/edit.html")
		if err != nil {
			panic(err)
		}
		err = tmplt.Execute(w, feed)
		if err != nil {
			panic(err)
		}
	}
}

func setEditFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}
		url := r.FormValue("URL")

		rss, err := rss(url)
		if err != nil {
			panic(err)
		}

		feed := Feed{
			Id:      idFormValue(r),
			URL:     url,
			Channel: rss.Channels[0],
		}
		if feed.Id == 0 {
			err := d.
				QueryRowContext(r.Context(), `
				INSERT INTO feeds 
					(url, title, description, link) VALUES 
					($1, $2, $3, $4)
				RETURNING id`, feed.URL, feed.Title, feed.Description, feed.Link).
				Scan(&feed.Id)
			if err != nil {
				panic(err)
			}
		} else {
			_, err := d.
				ExecContext(r.Context(), `
				UPDATE feeds 
				SET 
					title=$1,
					url=$2,link=$3,
					description=$4
				WHERE id=$5`, feed.Title, feed.URL, feed.Link, feed.Description, feed.Id)
			if err != nil {
				panic(err)
			}
		}

		// TODO run in tx
		_, err = d.
			ExecContext(r.Context(), `
			DELETE FROM feed_entries
			WHERE feed_id = $1`, feed.Id)
		if err != nil {
			panic(err)
		}
		for _, item := range rss.Channels[0].Items {
			feedEntry := FeedEntry{
				FeedId: feed.Id,
				Item:   item,
			}

			_, err = d.
				ExecContext(r.Context(), `
				INSERT INTO feed_entries
					(feed_id, title, description, link) VALUES
					($1, $2, $3, $4)`, feedEntry.FeedId, feedEntry.Title, feedEntry.Description, feedEntry.Link)
			if err != nil {
				panic(err)
			}
		}

		err = redirect(w, "feed updated")
		if err != nil {
			panic(err)
		}
	}
}

func deleteFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		_, err := d.
			ExecContext(r.Context(), "DELETE FROM feeds WHERE id = $1", r.PathValue("Id"))
		if err != nil {
			panic(err)
		}

		err = redirect(w, "feed deleted")
		if err != nil {
			panic(err)
		}
	}
}
