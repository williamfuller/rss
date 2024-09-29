package main

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"net/http"
	"os"
)

type Feed struct {
	Id  int
	URL string
	Channel
}

type FeedsController struct{}

func (f *FeedsController) Show(d *sql.DB, w http.ResponseWriter, r *http.Request) {
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

func (f *FeedsController) GetEdit(d *sql.DB, w http.ResponseWriter, r *http.Request) {
	var feed Feed
	err := d.
		QueryRowContext(r.Context(), "SELECT id, url FROM feeds WHERE id = $1", idPathValue(r)).
		Scan(&feed.Id, &feed.URL)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		panic(err)
	}

	nav, err := os.ReadFile("components/nav.html")
	if err != nil {
		panic(err)
	}

	pageElements := struct {
		Nav  template.HTML
		Feed Feed
	}{
		Nav:  template.HTML(nav),
		Feed: feed,
	}

	tmplt, err := template.ParseFiles("templates/edit.html")
	if err != nil {
		panic(err)
	}
	err = tmplt.Execute(w, pageElements)
	if err != nil {
		panic(err)
	}
}

func (f *Feed) update(d *sql.DB) error {
	rss, err := rss(f.URL)
	if err != nil {
		return err
	}

	f.Channel = rss.Channels[0]

	if f.Id == 0 {
		err := d.
			QueryRowContext(context.Background(), `
				INSERT INTO feeds 
					(url, title, description, link) VALUES 
					($1, $2, $3, $4)
				RETURNING id`, f.URL, f.Title, f.Description, f.Link).
			Scan(&f.Id)
		if err != nil {
			return err
		}
	} else {
		_, err := d.
			ExecContext(context.Background(), `
				UPDATE feeds 
				SET 
					title=$1,
					url=$2,link=$3,
					description=$4
				WHERE id=$5`, f.Title, f.URL, f.Link, f.Description, f.Id)
		if err != nil {
			return err
		}
	}

	// TODO run in tx
	_, err = d.
		ExecContext(context.Background(), `
			DELETE FROM feed_entries
			WHERE feed_id = $1`, f.Id)
	if err != nil {
		return err
	}

	for _, item := range rss.Channels[0].Items {
		feedEntry := FeedEntry{
			FeedId: f.Id,
			Item:   item,
		}

		_, err = d.
			ExecContext(context.Background(), `
				INSERT INTO feed_entries
					(feed_id, title, description,link, pub_date) VALUES
					($1, $2, $3, $4, $5)`, feedEntry.FeedId, feedEntry.Title, feedEntry.Description, feedEntry.Link, feedEntry.PubDate.Time)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *FeedsController) SetEdit(d *sql.DB, w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		panic(err)
	}

	feed := Feed{
		Id:  idFormValue(r),
		URL: r.FormValue("URL"),
	}
	err = feed.update(d)
	if err != nil {
		panic(err)
	}

	err = redirect(w, "feed updated")
	if err != nil {
		panic(err)
	}
}

func (f *FeedsController) Delete(d *sql.DB, w http.ResponseWriter, r *http.Request) {
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

func (f *FeedsController) List(d *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := d.
		QueryContext(r.Context(), "SELECT id, title from feeds")
	if err != nil {
		panic(err)
	}

	var feeds []Feed
	for {
		hasRow := rows.Next()
		if rows.Err() != nil {
			panic(rows.Err())
		}
		if !hasRow {
			break
		}

		var feed Feed
		err := rows.Scan(&feed.Id, &feed.Title)
		if err != nil {
			panic(err)
		}
		feeds = append(feeds, feed)
	}

	tmplt, err := template.ParseFiles("templates/feeds.html")
	if err != nil {
		panic(err)
	}
	err = tmplt.Execute(w, feeds)
	if err != nil {
		panic(err)
	}

}
