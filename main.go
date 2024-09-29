package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
)

type RFC1123Time struct {
	time.Time
}

func (t *RFC1123Time) String() string {
	return t.Time.Format(time.RFC1123)
}

func (t *RFC1123Time) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var rfc822TimeString string
	err := d.DecodeElement(&rfc822TimeString, &start)
	if err != nil {
		return err
	}

	parsedTime, err := time.Parse(time.RFC1123, rfc822TimeString)
	if err != nil {
		return err
	}

	t.Time = parsedTime

	return nil
}

type Rss struct {
	XMLName  xml.Name  `xml:"rss"`
	Channels []Channel `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []Item `xml:"item"`
}

type Item struct {
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description template.HTML `xml:"description"`
	PubDate     RFC1123Time   `xml:"pubDate"`
}

func rss(link string) (*Rss, error) {
	resp, err := http.Get(link)
	if err != nil {
		return nil, fmt.Errorf("rss http get: %w", err)
	}

	var rss *Rss
	if resp.StatusCode == 200 {
		err := xml.NewDecoder(resp.Body).Decode(&rss)
		if err != nil {
			return nil, fmt.Errorf("rss xml decode: %w", err)
		}
		defer resp.Body.Close()
	}

	return rss, nil
}

func redirect(w http.ResponseWriter, message string) error {
	tmplt, err := template.ParseFiles("templates/redirect.html")
	if err != nil {
		return err
	}
	tmplt.Execute(w, message)
	return nil
}

func route(path string, d *sql.DB, controller func(*sql.DB, http.ResponseWriter, *http.Request)) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		controller(d, w, r)
	})
}

func Index(d *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := d.Query(`
	SELECT id, title, link, pub_date 
	FROM feed_entries 
	ORDER by pub_date DESC, title`)
	if err != nil {
		panic(err)
	}

	var feedEntries []FeedEntry
	for {
		var feedEntry FeedEntry
		if rows.Next() {
			err := rows.Scan(&feedEntry.Id, &feedEntry.Title, &feedEntry.Link, &feedEntry.PubDate.Time)
			if err != nil {
				panic(err)
			}

			feedEntries = append(feedEntries, feedEntry)
		} else {
			if rows.Err() != nil {
				panic(rows.Err())
			}
			break
		}
	}

	tmplt, err := template.ParseFiles("templates/index.html")
	if err != nil {
		panic(err)
	}

	err = tmplt.Execute(w, feedEntries)
	if err != nil {
		panic(err)
	}
}

func startWebServer(db *sql.DB) error {
	var feeds FeedsController
	var feedEntries FeedEntriessController
	route("GET /{$}", db, Index)
	route("GET /feed_entries/show/{Id}", db, feedEntries.Show)
	route("GET /feeds/edit/{Id}", db, feeds.GetEdit)
	route("GET /feeds/edit", db, feeds.GetEdit)
	route("POST /feeds/edit", db, feeds.SetEdit)
	route("GET /feeds/delete/{Id}", db, feeds.Delete)
	route("GET /feeds/show/{Id}", db, feeds.Show)
	route("GET /feeds/list", db, feeds.List)

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
