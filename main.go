package main

import (
	"bytes"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"regexp"

	_ "github.com/lib/pq"
)

type HTML template.HTML

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
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
}

type Feed struct {
	Id  int
	URL string
	Channel
}

type FeedEntry struct {
	Id     int
	FeedId int
	Item
}

func (i *Item) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {

	var startElement xml.StartElement
	for {
		token, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			panic(err)
		}
		switch token.(type) {
		case xml.StartElement:
			startElement = token.(xml.StartElement)
		case xml.CharData:
			charData := token.(xml.CharData)
			switch startElement.Name.Local {
			case "title":
				i.Title += string(bytes.TrimSpace(charData))
			case "link":
				i.Link += string(bytes.TrimSpace(charData))
			case "description":
				i.Description += regexp.MustCompile("<[^>]*>").ReplaceAllString(string(charData), "\n")
			}
		case xml.EndElement:
			break
		}
	}

	return nil
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

func index(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var feeds []Feed
		rows, err := d.Query("SELECT id, title FROM feeds")
		if err != nil {
			panic(err)
		}
		for {
			if rows.Next() {
				var feed Feed
				err := rows.Scan(&feed.Id, &feed.Title)
				if err != nil {
					panic(err)
				}
				feeds = append(feeds, feed)
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

		err = tmplt.Execute(w, feeds)
		if err != nil {
			panic(err)
		}
	}
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
		http.HandleFunc("GET /{$}", index(db))
		http.HandleFunc("GET /feeds/edit/{Id}", getEditFeed(db))
		http.HandleFunc("GET /feeds/edit", getEditFeed(db))
		http.HandleFunc("POST /feeds/edit", setEditFeed(db))
		http.HandleFunc("GET /feeds/delete/{Id}", deleteFeed(db))
		http.HandleFunc("GET /feeds/show/{Id}", showFeed(db))
		http.Handle("/static/", http.FileServer(http.Dir("")))
		err = http.ListenAndServe(":8080", nil)
		if err != nil {
			panic(err)
		}
	}
}
