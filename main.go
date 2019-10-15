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

// Using SKV just to make it easier on ourselves
var store *skv.KVStore
var err error

var baseURL = "https://anoni.sh/"

func main() {
	// Open the store
	store, err = skv.Open("sessions.db")
	if err != nil {
		panic(err)
	}
	defer store.Close()

	r := mux.NewRouter()
	// Set up endpoints
	r.HandleFunc("/", index).Methods("GET")
	r.HandleFunc("/add", addRedirect).Methods("POST")
	r.HandleFunc("/checkRedirect", checkRedirect).Methods("POST")
	r.HandleFunc("/{key}", redirect).Methods("GET")

	// Autocert makes it easy to manage SSL certs
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

// If a redirect exsits, forward to it, if not forward them to our primary URL
func redirect(w http.ResponseWriter, r *http.Request) {
	// Read input and convert the passed key to valid UTF8
	vars := mux.Vars(r)
	redirectKey := strings.ToValidUTF8(vars["key"], "")

	// Get redirect URL from key-store if there isn't any target for the key
	// send off to the homepage, otherwise follow the redirect
	var redirectTo string
	err := store.Get(redirectKey, &redirectTo)
	if err != nil || redirectTo == "" {
		http.Redirect(w, r, baseURL, http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
	return
}

// Check if a redirect exists, if it does display the forward URL to the user
func checkRedirect(w http.ResponseWriter, r *http.Request) {
	// Get the URL post parameter, and stripe the {baseURL} from the URL to isolate the key
	redirectKey := strings.ToValidUTF8(r.FormValue("url"), "")
	redirectKey = strings.Replace(redirectKey, baseURL, "", 1)

	// Check where the key points to, if it's invalid, tell the user. If it is valid send the
	// URL to the user
	var redirectTo string
	err := store.Get(redirectKey, &redirectTo)
	if err != nil || redirectTo == "" {
		w.Write([]byte("Invalid redirect key"))
		return
	}

	w.Write([]byte(redirectTo))
	return
}

// Add a redirect key
func addRedirect(w http.ResponseWriter, r *http.Request) {
	// Get the POST params, and convert them both to UTF-8
	redirectKey := strings.ToValidUTF8(r.FormValue("key"), "")
	redirectTo := strings.ToValidUTF8(r.FormValue("to"), "")

	// Verify parameters aren't empty
	if redirectKey == "" || redirectTo == "" {
		w.Write([]byte("Missing parameters"))
		return
	}

	// Check if key already redirects to a URL, if it does let the user know
	var currentRedirect string
	store.Get(redirectKey, &currentRedirect)
	if currentRedirect != "" {
		w.Write([]byte("Key already taken."))
		return
	}

	// If the "URL" doesn't contain :// (http:// || https:// || ftp:// etc) just
	// add http:// to the beginning so it's a proper redirect
	if !strings.Contains(redirectTo, "://") {
		redirectTo = "http://" + redirectTo
	}

	// If it does contain :// but doesn't match a proper URL let the user know
	_, err = url.ParseRequestURI(redirectTo)
	if err != nil {
		w.Write([]byte("Not a valid URL."))
		return
	}

	// Store the new key/value to the store
	err = store.Put(redirectKey, redirectTo)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Pass the shortened URL to the user
	redirectKey = url.QueryEscape(redirectKey)
	w.Write([]byte(baseURL + redirectKey))
	return

}
