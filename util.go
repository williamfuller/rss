package main

import (
	"net/http"
	"strconv"
)

func idFormValue(r *http.Request) int {
	id, err := strconv.Atoi(r.FormValue("Id"))
	if err != nil {
		id = 0
	}

	return id
}

func idPathValue(r *http.Request) int {
	id, err := strconv.Atoi(r.PathValue("Id"))
	if err != nil {
		id = 0
	}

	return id
}
