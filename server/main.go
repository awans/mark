package server

import (
	"net/http"

	"github.com/awans/mark/app"
	"github.com/awans/mark/server/api"
	"github.com/gorilla/mux"
	"github.com/PuerkitoBio/goquery"
	"github.com/kennygrant/sanitize"

)

// TitleHandler returns the page title of a url
func TitleHandler(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query()["url"][0]

	doc, err := goquery.NewDocument(url)
	// TODO
	if err != nil {
		panic(err)
	}
	title := doc.Find("title").Text()
	sanitized := sanitize.HTML(title)
	w.Write([]byte(sanitized))
}

// New returns a new mark server
func New(db *app.DB) http.Handler {
	r := mux.NewRouter()

	apiRouter := r.PathPrefix("/api").Subrouter()

	f := api.NewFeed(db)
	apiRouter.HandleFunc("/feed", f.GetFeed).Methods("GET")

	b := api.NewBookmark(db)
	apiRouter.HandleFunc("/bookmark", b.AddBookmark).Methods("POST")

	d := api.NewDebug(db)
	apiRouter.HandleFunc("/debug", d.GetDebug).Methods("GET")

	r.HandleFunc("/title", TitleHandler).Methods("GET")

	r.Handle("/{path:.*}", http.FileServer(http.Dir("server/data/static/build")))
	return r
}
