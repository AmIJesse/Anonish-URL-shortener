package main

import (
	"crypto/tls"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/acme/autocert"

	"github.com/rapidloop/skv"
)

// Creation rate is used to keep track of how many keys a user has created in the last hour
type creationRate struct {
	IP        string
	creations int
}

// The rate limiter keeps a list of creationRates, and a mutex to lock it during read/modify
type rateLimiter struct {
	sync.RWMutex
	rates map[string]int
}

// tmplIndex hold template data for the index page, currently only the URL
type tmplIndex struct {
	URL    string
	TITLE  string
	HEADER string
}

// Analytics Tracker keeps track of how many keys are created in the past 24 hours, and how
// many redirects have been followed in the last 24 hours (Only keeping track of timestamps,
// no URL data)
type analyticsTracker struct {
	sync.RWMutex
	dailyCreations []time.Time
	dailyRedirects []time.Time
}

// Using SKV just to make it easier on ourselves
var (
	store      *skv.KVStore
	err        error
	baseURL    = "https://anoni.sh/"
	htmlTitle  = "Anoni.sh URL Shortener"
	htmlHeader = "anoni.sh"

	rateLimit = rateLimiter{}
	maxRate   = 10

	analytics = analyticsTracker{}
	adminKey  string

	invalidKeyChars = []string{"/", "\\", "\"", ":", "*", "?", "<", ">"} // Keys that cause errors on redirection
)

// resetRateLimitHourly will reset our IP rate limits every hour
func resetRateLimitsHourly() {
	for {
		time.Sleep(1 * time.Hour)
		rateLimit.Lock()
		rateLimit.rates = make(map[string]int)
		rateLimit.Unlock()
	}
}

func main() {
	// Open the store
	store, err = skv.Open("sessions.db")
	if err != nil {
		panic(err)
	}
	defer store.Close()

	adminKey = os.Getenv("anoniAdminKey")
	if adminKey == "" {
		panic("Missing environment key \"anoniAdminKey\"")
	}

	// Initialize the rate limits, and start the hourly resetter
	rateLimit.rates = make(map[string]int)
	go resetRateLimitsHourly()

	r := mux.NewRouter()
	// Set up endpoints
	r.HandleFunc("/", index).Methods("GET")
	r.HandleFunc("/add", addRedirect).Methods("POST")
	r.HandleFunc("/checkRedirect", checkRedirect).Methods("POST")
	r.HandleFunc("/stats", stats).Methods("GET")
	r.HandleFunc("/{key}", redirect).Methods("GET")

	fs := http.StripPrefix("/static/", http.FileServer(http.Dir("./assets/")))
	r.PathPrefix("/static/").Handler(fs)
	go mime.AddExtensionType(".js", "application/javascript; charset=utf-8")

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

	htmlData := tmplIndex{
		URL:    baseURL,
		TITLE:  htmlTitle,
		HEADER: htmlHeader,
	}
	tmpl, err := template.ParseFiles("index.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, htmlData)
}

// Stats will show a user (if they provide the proper key) basic usage stats
// such as how many new keys have beencreated in the past day, and how many
// users have followed a shortened link. No key/link data provided
func stats(w http.ResponseWriter, r *http.Request) {
	// Get the "key" value passed as a GET param (I know), if it's not included
	// pass it as a redirect, if it is included but incorrect redirect them to
	// the homepage
	key := r.FormValue("key")
	if key == "" {
		redirect(w, r)
		return
	}
	if key != adminKey {
		http.Redirect(w, r, baseURL, http.StatusSeeOther)
		return
	}

	// Loop over the dailyCreations and dailyRedirect values of our analyics struct
	// and only save the ones that are from the last 24 hours, and save the ammount
	analytics.Lock()
	var newCreationsToday []time.Time
	var newRedirectsToday []time.Time

	for _, v := range analytics.dailyCreations {
		if time.Since(v) < (time.Hour * 24) {
			newCreationsToday = append(newCreationsToday, v)
		}
	}
	analytics.dailyCreations = newCreationsToday
	creationsToday := len(analytics.dailyCreations)

	for _, v := range analytics.dailyRedirects {
		if time.Since(v) < (time.Hour * 24) {
			newRedirectsToday = append(newRedirectsToday, v)
		}
	}
	analytics.dailyRedirects = newRedirectsToday
	redirectsToday := len(analytics.dailyRedirects)
	analytics.Unlock()

	// Print the results to the user
	w.Write([]byte(fmt.Sprintf("Creations today: %d\nRedirects today: %d\n",
		creationsToday, redirectsToday)))
	return
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

	analytics.Lock()
	analytics.dailyRedirects = append(analytics.dailyRedirects, time.Now())
	analytics.Unlock()

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
	return
}

// Check if a redirect exists, if it does display the forward URL to the user
func checkRedirect(w http.ResponseWriter, r *http.Request) {
	// Get the URL post parameter, and stripe the {baseURL} from the URL to isolate the key
	redirectKey := strings.ToValidUTF8(r.FormValue("url"), "")
	redirectKey = strings.Replace(redirectKey, baseURL, "", 1)

	// Check where the key points to, if it's invalid, tell the user. If it is valid send
	// the URL to the user
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

	// Strip invalid characters out of the redirect key
	for _, v := range invalidKeyChars {
		redirectKey = strings.ReplaceAll(redirectKey, v, "")
	}

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

	// check user rate limit, if they have saved {maxRate} keys this hour, let them know
	userIP := strings.Split(r.RemoteAddr, ":")[0]
	rateLimit.Lock()
	defer rateLimit.Unlock()
	userRate := rateLimit.rates[userIP]
	if userRate >= maxRate {
		w.Write([]byte(fmt.Sprintf("Hit limit of %d shortens per hour.", maxRate)))
		return
	}
	rateLimit.rates[userIP] = userRate + 1

	// Add a new creation timestamp to the "analytics"
	analytics.Lock()
	analytics.dailyCreations = append(analytics.dailyCreations, time.Now())
	analytics.Unlock()

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
