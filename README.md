# Anonish URL shortener

Anonish is a private URL shortener, with absolutely no logging. Check it out at https://anoni.sh.

# Features!
  - Create a shortened URL with a custom shortening "key"
  - Instant redirect via status-codes, no JS redirecting
  - Doesn't save *any* logs for user interaction

### Tech

Anonish uses a couple open source repositories to work properly:

* [mux] - A powerful HTTP router and URL matcher for building Go web servers
* [autocert] - Automated letsencrypt SSL Certs
* [skv] - Super easy to use key-value store

### Installation
Installation is simple, first install golang version 1.13+
```bash
git clone https://github.com/AmIJesse/Anonish-URL-shortener
cd Anonish-URL-shortener
nano main.go
```
Edit the line that contains
```
var baseURL = "https://anoni.sh/"
```
and change it to whatever your URL will be, and save the file
```
nano index.html
```
change the line that contains
```
Redirect Key (https://anoni.sh/{key}):<br>
```
and change that to your URL as well, and save the file
```
go get
go run main.go
```
While running it this way, it will only run while your current termial session is active, I recommend building the binary and creating a service for it, or running in in a tmux/screen session. You can build it wilth
```
go build
```


### Todo
 - Rewrite the HTML so it looks cleaner
 - Add templating so we have no "hardcoded" URL in the html file

License
----

MIT

[//]: # (These are reference links used in the body of this note and get stripped out when the markdown processor does its job. There is no need to format nicely because it shouldn't be seen. Thanks SO - http://stackoverflow.com/questions/4823468/store-comments-in-markdown-syntax)


   [mux]: <github.com/gorilla/mux>
   [autocert]: <golang.org/x/crypto/acme/autocert>
   [skv]: <github.com/rapidloop/skv>
