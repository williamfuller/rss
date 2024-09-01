package main

import (
	"database/sql"
	"html/template"
	"net/http"
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

func getEditFeed(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var feed Feed

		id := r.PathValue("Id")
		if id != "" {
			err := d.
				QueryRowContext(r.Context(), "SELECT id, name, url FROM feeds WHERE id = $1", id).
				Scan(&feed.Id, &feed.Name, &feed.URL)
			if err != nil {
				panic(err)
			}
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

		id := r.FormValue("Id")
		if id == "0" {
			_, err = d.
				ExecContext(r.Context(), "INSERT INTO feeds (name, url) VALUES ($1, $2)", r.FormValue("Name"), r.FormValue("URL"))
		} else {
			_, err = d.
				ExecContext(r.Context(), "UPDATE feeds SET name=$1, url=$2 WHERE id=$3", r.FormValue("Name"), r.FormValue("URL"), id)
		}
		if err != nil {
			panic(err)
		}

		if id != "0" {
			err = redirect(w, "feed updated ")
		} else {
			err = redirect(w, "feed added")
		}
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
