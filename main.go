package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dyatlov/go-opengraph/opengraph"
	"tailscale.com/client/tailscale"
	"tailscale.com/tsnet"
)

type Bookmark struct {
	Id          int
	Title       string
	SiteName    string
	Link        string
	Description string
	ImageUrl    string
}

// data for index template
type Data struct {
	RemoteHost string
	RemoteUser string
	RemoteAddr string
	Bookmarks  []Bookmark
}

var lc *tailscale.LocalClient

// opengraph -> bookmark struct
func ogToBookmark(og *opengraph.OpenGraph) Bookmark {
	return Bookmark{
		Title:       og.Title,
		SiteName:    og.SiteName,
		Description: og.Description,
	}
}

// routing
func serveHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("GET /style.css", handleCss)
	mux.HandleFunc("/save", handleSave)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	})
}

// handle saving link (GET or POST)
func handleSave(w http.ResponseWriter, r *http.Request) {
	var link string

	switch r.Method {
	case "GET":
		link = r.FormValue("link")
	case "POST":
		link = r.PostFormValue("link")
	default:
		http.Error(w, "invalid method for /save", http.StatusMethodNotAllowed)

	}
	if link == "" {
		http.Error(w, "no link param specified bruh", 400)
		return
	}

	c := http.Client{
		Timeout: time.Second * 6,
	}

	res, err := c.Get(link)
	if err != nil {
		log.Printf("error getting link %q: %v", link, err)
		http.Error(w, err.Error(), 500)
		return
	}
	o := opengraph.NewOpenGraph()
	o.ProcessHTML(res.Body)
	res.Body.Close()

	mark := ogToBookmark(o)
	mark.Link = link

	err = dbSave(mark)
	if err != nil {
		log.Printf("error writing link to db %q: %v", link, err)
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return
}

// handle index page (list of bookmarks)
func handleIndex(w http.ResponseWriter, r *http.Request) {
	who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	marks, err := dbGetAll()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Connection", "close")

	tmpData := Data{
		RemoteUser: who.UserProfile.LoginName,
		RemoteHost: who.Node.ComputedName,
		RemoteAddr: r.RemoteAddr,
		Bookmarks:  marks,
	}

	t, err := template.ParseFiles("./index.tmpl.html")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	log.Printf("got everything... executing template")
	err = t.Execute(w, tmpData)
	if err != nil {
		log.Printf("error executing template: %v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	log.Printf("finished execution")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	return
}

func handleCss(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./style.css")
}

var (
	hostname string
	dbPath   string
)

func init() {
	flag.StringVar(&hostname, "hostname", "bmark", "hostname for tailnet")
	flag.StringVar(&dbPath, "d", "./bookmarks.db", "path to boltdb file")

	flag.Parse()
}

func main() {
	srv := &tsnet.Server{
		Hostname:   hostname,
		Logf:       log.Printf,
		Ephemeral:  false,
		ControlURL: os.Getenv("TS_URL"),
		AuthKey:    os.Getenv("TS_AUTHKEY"),
	}
	ln, err := srv.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	lc, err = srv.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	if err = http.Serve(ln, serveHandler()); err != nil {
		log.Fatal(err)
	}
}
