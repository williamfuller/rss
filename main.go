package main

import (
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

func index(d *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := d.Query("SELECT title, link, description, pub_date FROM feed_entries ORDER by pub_date DESC, title")
	if err != nil {
		panic(err)
	}

	var feedEntries []FeedEntry
	for {
		var feedEntry FeedEntry
		if rows.Next() {
			err := rows.Scan(&feedEntry.Title, &feedEntry.Link, &feedEntry.Description, &feedEntry.PubDate.Time)
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

func route(path string, d *sql.DB, controller func(*sql.DB, http.ResponseWriter, *http.Request)) {
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		controller(d, w, r)
	})
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
		var feeds FeedsController
		route("GET /{$}", db, index)
		route("GET /feeds/edit/{Id}", db, feeds.GetEdit)
		route("GET /feeds/edit", db, feeds.GetEdit)
		route("POST /feeds/edit", db, feeds.SetEdit)
		route("GET /feeds/delete/{Id}", db, feeds.Delete)
		route("GET /feeds/show/{Id}", db, feeds.Show)
		route("GET /feeds/list", db, feeds.List)

		http.Handle("/static/", http.FileServer(http.Dir("")))

		err = http.ListenAndServe(":8080", nil)
		if err != nil {
			panic(err)
		}
	}
}
