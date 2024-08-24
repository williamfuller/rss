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

	_ "github.com/lib/pq"

	"github.com/microcosm-cc/bluemonday"
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
	Title       string        `xml:"title"`
	Link        string        `xml:"link"`
	Description template.HTML `xml:"description"`
}

type Feed struct {
	Id   int
	URL  string
	Name string
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
				p := bluemonday.UGCPolicy()
				i.Description += template.HTML(p.SanitizeBytes(charData))
			}
		case xml.EndElement:
			break
		}
	}

	return nil
}

func rss(url string) (*Rss, error) {
	resp, err := http.Get(url)
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
	tmplt, err := template.ParseFiles("redirect.html")
	if err != nil {
		return err
	}
	tmplt.Execute(w, message)
	return nil
}

func index(d *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var feeds []Feed
		rows, err := d.Query("SELECT id, name FROM feeds;")
		if err != nil {
			panic(err)
		}
		for {
			if rows.Next() {
				var feed Feed
				err := rows.Scan(&feed.Id, &feed.Name)
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

		tmplt, err := template.ParseFiles("index.html")
		if err != nil {
			panic(err)
		}
		tmplt.Execute(w, feeds)
	}
}

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

		tmplt, err := template.ParseFiles("feed.html")
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

		tmplt, err := template.ParseFiles("edit.html")
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

func newDB() (*sql.DB, error) {
	const DatabaseURL = "postgres://postgres@:5432/rss?sslmode=disable"
	return sql.Open("postgres", DatabaseURL)
}

func main() {
	db, err := newDB()
	if err != nil {
		panic(err)
	}

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
