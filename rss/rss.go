package rss

import (
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"
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

func New(link string) (*Rss, error) {
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
