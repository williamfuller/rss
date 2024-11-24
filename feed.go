package main

import (
	"context"
	"database/sql"
	"errors"
	"html/template"
	"net/http"
	"rss-app/rss"
)

type Feed struct {
	Id       int
	URL      string
	IsHidden bool
	rss.Channel
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
		var item rss.Item
		err := rows.Scan(&title, &item.Title, &item.Link, &item.Description)
		if err != nil {
			panic(err)
		}

		feed.Title = title
		feed.Items = append(feed.Items, item)
	}

	tmplt, err := template.ParseFiles("pages/feed.html")
	if err != nil {
		panic(err)
	}

	err = tmplt.Execute(w, feed)
	if err != nil {
		panic(err)
	}
}

func (f *FeedsController) GetEdit(d *sql.DB, w http.ResponseWriter, r *http.Request) (*Response, string, error) {
	var feed Feed
	err := d.
		QueryRowContext(r.Context(), "SELECT id, url, is_hidden FROM feeds WHERE id = $1", idPathValue(r)).
		Scan(&feed.Id, &feed.URL, &feed.IsHidden)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, "", err
	}

	return &Response{Data: feed, ShowFilter: false}, "html/feeds/edit.html", nil
}

func (f *Feed) update(d *sql.DB) error {
	rss, err := rss.New(f.URL)
	if err != nil {
		return err
	}

	f.Channel = rss.Channels[0]

	if f.Id == 0 {
		err := d.
			QueryRowContext(context.Background(), `
				INSERT INTO feeds 
					(url, is_hidden, title, description, link) VALUES 
					($1, $2, $3, $4, $5)
				RETURNING id`, f.URL, f.IsHidden, f.Title, f.Description, f.Link).
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
					description=$4,
					is_hidden=$5
				WHERE id=$6`, f.Title, f.URL, f.Link, f.Description, f.IsHidden, f.Id)
		if err != nil {
			return err
		}
	}

	_, err = d.
		ExecContext(context.Background(), `
			DELETE FROM feed_entries
			WHERE 
				feed_id = $1 AND
				pub_date < NOW() - interval '30 days'
			`, f.Id)
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
					(feed_id, title, description, link, pub_date) 
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT ON CONSTRAINT feed_id_link_key DO NOTHING
					`, feedEntry.FeedId, feedEntry.Title, feedEntry.Description, feedEntry.Link, feedEntry.PubDate.Time)
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *FeedsController) SetEdit(d *sql.DB, w http.ResponseWriter, r *http.Request) (*Response, string, error) {
	feed := Feed{
		Id:       idFormValue(r),
		IsHidden: r.FormValue("IsHidden") == "on",
		URL:      r.FormValue("URL"),
	}
	err := feed.update(d)
	if err != nil {
		return nil, "", err
	}

	return Index(d, w, r)
}

func (f *FeedsController) Delete(d *sql.DB, w http.ResponseWriter, r *http.Request) (*Response, string, error) {
	_, err := d.
		ExecContext(r.Context(), "DELETE FROM feeds WHERE id = $1", r.PathValue("Id"))
	if err != nil {
		return nil, "", err
	}

	return &Response{Data: "feed deleted"}, "redirect", nil
}

func (f *FeedsController) List(d *sql.DB, w http.ResponseWriter, r *http.Request) (*Response, string, error) {
	rows, err := d.
		QueryContext(r.Context(), "SELECT id, title from feeds")
	if err != nil {
		return nil, "", err
	}

	var feeds []Feed
	for {
		hasRow := rows.Next()
		if rows.Err() != nil {
			return nil, "", rows.Err()
		}
		if !hasRow {
			break
		}

		var feed Feed
		err := rows.Scan(&feed.Id, &feed.Title)
		if err != nil {
			return nil, "", err
		}
		feeds = append(feeds, feed)
	}

	return &Response{Data: feeds}, "html/feeds/list.html", nil
}
