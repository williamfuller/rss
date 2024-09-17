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
	Title        string         `xml:"title"`
	Link         string         `xml:"link"`
	CommentsLink sql.NullString `xml:"comments"`
	Description  string         `xml:"description"`
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
			case "comments":
				str := string(bytes.TrimSpace(charData))
				valid := str != ""
				i.CommentsLink = sql.NullString{String: str, Valid: valid}
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

func index(d *sql.DB, w http.ResponseWriter, r *http.Request) {
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

		http.Handle("/static/", http.FileServer(http.Dir("")))

		err = http.ListenAndServe(":8080", nil)
		if err != nil {
			panic(err)
		}
	}
}
