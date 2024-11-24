package html

import "html/template"

func Parse(file string) *template.Template {
	return template.Must(
		template.New("layout.html").ParseFiles("html/layout.html", file))
}

func ParseWithFilter(file string) *template.Template {
	return template.Must(
		template.New("layout.html").ParseFiles("html/layout.html", "html/filter.html", file))
}
