package main

import (
	"database/sql"
	"errors"
	"html/template"
	"net/http"
	"strconv"
)

func showFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var url string
		err := d.
			QueryRowContext(r.Context(), "SELECT url FROM feeds WHERE id = $1", r.PathValue("Id")).
			Scan(&url)
		if err != nil {
			panic(err)
		}

		rss, err := rss(url)
		if err != nil {
			panic(err)
		}

		tmplt, err := template.ParseFiles("templates/feed.html")
		if err != nil {
			panic(err)
		}
		tmplt.Execute(w, rss)
	}
}

func idParam(r *http.Request) int {
	id, err := strconv.Atoi(r.FormValue("Id"))
	if err != nil {
		id = 0
	}

	return id
}

func getEditFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var feed Feed
		err := d.
			QueryRowContext(r.Context(), "SELECT id, url FROM feeds WHERE id = $1", idParam(r)).
			Scan(&feed.Id, &feed.URL)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			panic(err)
		}

		tmplt, err := template.ParseFiles("templates/edit.html")
		if err != nil {
			panic(err)
		}
		tmplt.Execute(w, feed)
	}
}

func setEditFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}

		feed := Feed{
			Id:  idParam(r),
			URL: r.FormValue("URL"),
		}

		rss, err := rss(feed.URL)
		if err != nil {
			panic(err)
		}
		feed.Name = rss.Channels[0].Title

		if feed.Id == 0 {
			_, err = d.
				ExecContext(r.Context(), "INSERT INTO feeds (name, url) VALUES ($1, $2)", feed.Name, feed.URL)
			if err != nil {
				panic(err)
			}

			err = redirect(w, "feed added")
			if err != nil {
				panic(err)
			}
		} else {
			_, err = d.
				ExecContext(r.Context(), "UPDATE feeds SET name=$1, url=$2 WHERE id=$3", feed.Name, feed.URL, feed.Id)
			if err != nil {
				panic(err)
			}

			err = redirect(w, "feed updated")
			if err != nil {
				panic(err)
			}
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
