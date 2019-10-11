package main

import (
	"crypto/tls"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme/autocert"

	"github.com/rapidloop/skv"
)

var store *skv.KVStore
var err error

var baseURL = "https://anoni.sh/"

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

	certManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache("certs"),
	}

	server := &http.Server{
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 120 * time.Second,
		Addr:         ":443",
		Handler:      r,
		TLSConfig: &tls.Config{
			GetCertificate: certManager.GetCertificate,
		},
	}

	//http.ListenAndServe(":8000", r) // For local testing only
	//os.Exit(0)
	go http.ListenAndServe(":80", certManager.HTTPHandler(nil))
	server.ListenAndServeTLS("", "")
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
	redirectKey := strings.ToValidUTF8(vars["key"], "")

	var redirectTo string
	err := store.Get(redirectKey, &redirectTo)
	if err != nil || redirectTo == "" {
		http.Redirect(w, r, baseURL, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
	return
}

// Add a redirect key
func addRedirect(w http.ResponseWriter, r *http.Request) {
	redirectKey := strings.ToValidUTF8(r.FormValue("key"), "")
	redirectTo := strings.ToValidUTF8(r.FormValue("to"), "")

	if redirectKey == "" || redirectTo == "" {
		w.Write([]byte("Missing parameters"))
		return
	}

	var currentRedirect string
	store.Get(redirectKey, &currentRedirect)
	if currentRedirect != "" {
		w.Write([]byte("Key already taken."))
		return
	}

	if !strings.Contains(redirectTo, "://") {
		redirectTo = "http://" + redirectTo
	}
	_, err = url.ParseRequestURI(redirectTo)
	if err != nil {
		w.Write([]byte("Not a valid URL."))
		return
	}

	err = store.Put(redirectKey, redirectTo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write([]byte(baseURL + redirectKey))
	return

}
