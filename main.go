package main

import (
	"database/sql"
	"github.com/brianglass/orthocal"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	// This is about the middle of the country including Hawaii and Alaska
	// Folks on the east coast won't much care after midnight and the folks in
	// Hawaii will have the day change happen at 9pm. That's not ideal, but my
	// guess is that most people won't be using the service after 9pm.
	TimeZone = "America/Los_Angeles"
)

var (
	TZ *time.Location
)

func init() {
	var e error
	TZ, e = time.LoadLocation(TimeZone)
	if e != nil {
		TZ = time.UTC
		log.Printf("Error loading '%s' timezone, using UTC.", TimeZone)
	}
}

func main() {
	var ocadb, bibledb *sql.DB
	var e error

	// Open up all the requisite databases

	if ocadb, e = sql.Open("sqlite3", "oca_calendar.db"); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}
	defer ocadb.Close()

	if bibledb, e = sql.Open("sqlite3", "kjv.db"); e != nil {
		log.Printf("Got error opening database: %#n. Exiting.", e)
		os.Exit(1)
	}
	defer bibledb.Close()

	bible := orthocal.NewBible(bibledb)

	// Setup HTTP routers

	router := mux.NewRouter()

	router.HandleFunc("/", homeHandler)

	ocaRouter := router.PathPrefix("/api/oca").Subrouter()
	NewCalendarServer(ocaRouter, ocadb, false, true, bible)

	echoRouter := router.NewRoute().Subrouter()
	NewSkill(echoRouter, os.Getenv("ALEXA_APP_ID"), ocadb, false, true, bible)

	// Launch the HTTP server
	router.Use(handlers.CompressHandler)
	// router.Use(logHeaderMiddleware)
	http.ListenAndServe(":8080", handlers.CombinedLoggingHandler(os.Stdout, router))
}

func homeHandler(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "text/html")

	t, e := template.ParseFiles("templates/home.html")
	if e != nil {
		http.Error(writer, "Template not found.", http.StatusInternalServerError)
		log.Println(e.Error())
		return
	}

	if e = t.Execute(writer, nil); e != nil {
		http.Error(writer, "Template rendering failed.", http.StatusInternalServerError)
		log.Println(e.Error())
		return
	}
}

func logHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		log.Println(request.Header)
		next.ServeHTTP(writer, request)
	})
}
