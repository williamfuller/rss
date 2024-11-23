package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"rss-app/html"
	"time"

	_ "github.com/lib/pq"
)

type Response struct {
	Data          interface{}
	ShowFilter    bool
	FilterOptions FilterOptions
}

type FilterOptions struct {
	Feeds []Feed
}

func route(path string, d *sql.DB, controller func(*sql.DB, http.ResponseWriter, *http.Request) (*Response, string, error), isPage bool) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			panic(err)
		}

		response, templateName, err := controller(d, w, r)
		if err != nil {
			panic(err)
		}

		if isPage {
			err = html.ParseWithFilter(templateName).Execute(w, &response)
		} else {
			err = html.Parse(templateName).Execute(w, &response)
		}
		if err != nil {
			panic(err)
		}
	})
}

func Index(db *sql.DB, w http.ResponseWriter, r *http.Request) (*Response, string, error) {
	feedId := r.FormValue("FeedId")
	var rows *sql.Rows
	var err error
	if feedId != "" {
		rows, err = db.Query(`
			SELECT id, title, link, description, pub_date 
			FROM feed_entries 
			WHERE feed_id = $1
			ORDER by pub_date DESC, title`,
			feedId)
	} else {
		rows, err = db.Query(`
			SELECT feed_entries.id, feed_entries.title, feed_entries.link, feed_entries.description, feed_entries.pub_date 
			FROM feed_entries, feeds
			WHERE feeds.id = feed_id 
				AND is_hidden = false
			ORDER by pub_date DESC, title`,
		)
	}
	if err != nil {
		return nil, "", err
	}

	var feedEntries []FeedEntry
	for {
		var feedEntry FeedEntry
		if rows.Next() {
			err := rows.Scan(&feedEntry.Id, &feedEntry.Title, &feedEntry.Link, &feedEntry.Description, &feedEntry.PubDate.Time)
			if err != nil {
				return nil, "", err
			}

			feedEntries = append(feedEntries, feedEntry)
		} else {
			if rows.Err() != nil {
				return nil, "", rows.Err()
			}
			break
		}
	}

	filterOptions, err := filterOptions(db)
	if err != nil {
		return nil, "", err
	}

	return &Response{Data: feedEntries, ShowFilter: true, FilterOptions: *filterOptions}, "html/feed_entries/list.html", nil
}

func filterOptions(db *sql.DB) (*FilterOptions, error) {
	rows, err := db.Query("SELECT id, title FROM feeds WHERE is_hidden = false")
	if err != nil {
		return nil, err
	}

	var filterOptions FilterOptions
	for {
		if !rows.Next() {
			if rows.Err() != nil {
				return nil, err
			}
			break
		}
		var feed Feed
		err = rows.Scan(&feed.Id, &feed.Title)
		if err != nil {
			return nil, err
		}
		filterOptions.Feeds = append(filterOptions.Feeds, feed)
	}

	return &filterOptions, nil
}

func startWebServer(db *sql.DB) error {
	var feeds FeedsController
	var feedEntries FeedEntriesController
	route("GET /{$}", db, Index, true)
	route("GET /feed_entries/show/{Id}", db, feedEntries.Show, false)
	route("GET /feeds/edit/{Id}", db, feeds.GetEdit, true)
	route("GET /feeds/edit", db, feeds.GetEdit, true)
	route("POST /feeds/edit", db, feeds.SetEdit, true)
	route("GET /feeds/delete/{Id}", db, feeds.Delete, true)
	route("GET /feeds/list", db, feeds.List, true)

	http.Handle("/static/", http.FileServer(http.Dir("")))

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	db, err := newDB()
	if err != nil {
		panic(err)
	}

	err = db.Ping()
	if err != nil {
		panic(err)
	}

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		err := migrate(db)
		if err != nil {
			panic(fmt.Errorf("migrate: %w", err))
		}
	} else {
		go func() {
			err := startWebServer(db)
			if err != nil {
				panic(err)
			}
		}()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rows, err := db.QueryContext(context.Background(), `
				SELECT id, url 
				FROM feeds 
				WHERE 
					update_at < NOW() AND
					is_updating = false 
				ORDER BY id 
				LIMIT 5`)
				if err != nil {
					panic(err)
				}

				for {
					hasRows := rows.Next()
					if !hasRows {
						if rows.Err() != nil {
							panic(rows.Err())
						}
						break
					}

					var feed Feed
					err := rows.Scan(&feed.Id, &feed.URL)
					if err != nil {
						panic(err)
					}

					go func(feed Feed) {
						_, err := db.Exec("UPDATE feeds SET is_updating = true WHERE id = $1", feed.Id)
						if err != nil {
							panic(err)
						}

						err = feed.update(db)
						if err != nil {
							panic(err)
						}

						_, err = db.Exec("UPDATE feeds SET is_updating = false, update_at = NOW() + interval '10 minutes' WHERE id = $1", feed.Id)
						if err != nil {
							panic(err)
						}
					}(feed)
				}
			}
		}
	}
}
