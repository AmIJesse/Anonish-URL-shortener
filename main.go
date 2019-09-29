package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/rapidloop/skv"
)

var store *skv.KVStore
var err error

func main() {
	// open the store
	store, err = skv.Open("sessions.db")
	if err != nil {
		panic(err)
	}
	defer store.Close()

	r := mux.NewRouter()
	//Set up endpoints
	r.HandleFunc("/", index).Methods("GET")
	r.HandleFunc("/add", addRedirect).Methods("POST")
	r.HandleFunc("/{key}", redirect).Methods("GET")

	// For testing purposes
	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8000",
		WriteTimeout: 5 * time.Second,
		ReadTimeout:  5 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

// index handles returning the HTML of index.html
func index(w http.ResponseWriter, r *http.Request) {
	indexHTML, err := ioutil.ReadFile("index.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(indexHTML)
}

// Check if the key exists, if not forward them to our primary URL
func redirect(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	redirectKey := strings.ToValidUTF8(vars["key"], nil)

	var redirectTo string
	err := store.Get(redirectKey, &redirectTo)
	if err != nil || redirectTo == "" {
		http.Redirect(w, r, "https://anon.ish/", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
	return
}

// Add a redirect key
func addRedirect(w http.ResponseWriter, r *http.Request) {
	redirectKey := strings.ToValidUTF8(r.FormValue("key"), nil)
	redirectTo := strings.ToValidUTF8(r.FormValue("to"), nil)

	if redirectKey == "" || redirectTo == "" {
		w.Write([]byte("Missing parameters"))
		return
	}

	err := url.ParseRequestURI(redirectTo)
	if err != nil {
		w.Write([]byte("Not a valid URL."))
		return
	}

	err := store.Put(redirectKey, redirectTo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "https://anon.ish/", http.StatusSeeOther)
	return

}
