package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
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

	var parsedTime time.Time
	parsedTime, err = time.Parse(time.RFC1123, rfc822TimeString)
	if err != nil {
		var err2 error
		parsedTime, err2 = time.Parse("Mon, _2 Jan 2006 15:04:05 -0700", rfc822TimeString)
		if err2 != nil {
			return errors.Join(err, err2)
		}
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

type Response struct {
	Data          interface{}
	ShowFilter    bool
	FilterOptions FilterOptions
}

type FilterOptions struct {
	Feeds []Feed
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
			err = template.Must(template.ParseFiles("templates/filter.html", "templates/nav.html", fmt.Sprintf("pages/%s.html", templateName))).
				ExecuteTemplate(w, templateName, &response)
		} else {
			err = template.Must(template.ParseFiles(fmt.Sprintf("template/%s.html", templateName))).
				ExecuteTemplate(w, templateName, &response)
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
			SELECT id, title, link, description, pub_date 
			FROM feed_entries 
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

	return &Response{Data: feedEntries, ShowFilter: true, FilterOptions: *filterOptions}, "index", nil
}

func filterOptions(db *sql.DB) (*FilterOptions, error) {
	rows, err := db.Query("SELECT id, title FROM feeds")
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
